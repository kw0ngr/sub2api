package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIResponsesTTFTStartsAtVisibleOutput(t *testing.T) {
	visibleEvents := []struct {
		name string
		line string
	}{
		{name: "text", line: `data: {"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"hello"}`},
		{name: "reasoning", line: `data: {"type":"response.reasoning_summary_text.delta","output_index":0,"summary_index":0,"delta":"think"}`},
		{name: "tool", line: `data: {"type":"response.function_call_arguments.done","output_index":0,"arguments":"{\"cmd\":\"ls\"}"}`},
	}
	for _, visible := range visibleEvents {
		for _, passthrough := range []bool{false, true} {
			t.Run(visible.name+"/"+streamModeName(passthrough), func(t *testing.T) {
				// Given: metadata and item-open frames arrive before any visible output.
				visibleDelay := 80 * time.Millisecond
				body := []string{
					`data: {"type":"response.created","response":{"id":"resp_visible","status":"in_progress"}}`,
					`data: {"type":"response.in_progress","response":{"id":"resp_visible","status":"in_progress"}}`,
					`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","status":"in_progress"}}`,
					visible.line,
					`data: {"type":"response.completed","response":{"id":"resp_visible","status":"completed","usage":{"input_tokens":1,"output_tokens":1}}}`,
				}

				// When: the stream is handled.
				firstTokenMs, err := runTask14StreamingFrames(t, passthrough, visibleDelay, body)

				// Then: TTFT starts at visible output, not the earlier metadata frames.
				require.NoError(t, err)
				require.NotNil(t, firstTokenMs)
				require.GreaterOrEqual(t, *firstTokenMs, int(visibleDelay.Milliseconds()/2))
			})
		}
	}
}

func TestOpenAIResponsesFirstTokenIgnoresUsageOnlyTerminal(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		t.Run(streamModeName(passthrough), func(t *testing.T) {
			// Given: a valid zero-token terminal usage frame without visible output.
			body := []string{
				`data: {"type":"response.created","response":{"id":"resp_usage","status":"in_progress"}}`,
				`data: {"type":"response.completed","response":{"id":"resp_usage","status":"completed","usage":{"input_tokens":0,"output_tokens":0}}}`,
			}

			// When: the stream is handled.
			firstTokenMs, err := runTask14StreamingFrames(t, passthrough, 0, body)

			// Then: zero-token usage is a valid success, but it is not TTFT.
			require.NoError(t, err)
			require.Nil(t, firstTokenMs)
		})
	}
}

func TestOpenAIResponsesEmptyCompletedFailsOverBeforeOutput(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		t.Run(streamModeName(passthrough), func(t *testing.T) {
			// Given: upstream ends with an empty terminal event and no usable output.
			gin.SetMode(gin.TestMode)
			svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid-empty-stream"}},
				Body: io.NopCloser(strings.NewReader(strings.Join([]string{
					`data: {"type":"response.created","response":{"id":"resp_empty","status":"in_progress"}}`,
					`data: {"type":"response.completed","response":{"id":"resp_empty","status":"completed"}}`,
				}, "\n"))),
			}

			// When: the stream is handled.
			var err error
			if passthrough {
				_, err = svc.handleStreamingResponsePassthrough(context.Background(), resp, c, task14OpenAIAccount(), time.Now())
			} else {
				_, err = svc.handleStreamingResponse(context.Background(), resp, c, task14OpenAIAccount(), time.Now(), "gpt-5.6-sol", "gpt-5.6-sol")
			}

			// Then: it returns a failover error before sending an empty success.
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
			require.False(t, c.Writer.Written())
			require.Empty(t, rec.Body.String())
		})
	}
}

func TestOpenAIResponsesEmptyCompletedNonStreamingFailure(t *testing.T) {
	// Given: a stream=false SSE response with an empty completed terminal frame.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := task14SSEResponse("rid-empty-nonstream")
	body := task14SSEBody(
		`data: {"type":"response.created","response":{"id":"resp_empty","status":"in_progress"}}`,
		`data: {"type":"response.completed","response":{"id":"resp_empty","status":"completed"}}`,
	)

	// When: the non-streaming converter handles it.
	usage, err := svc.handleSSEToJSON(resp, c, task14OpenAIAccount(), body, "gpt-5.6-sol", "gpt-5.6-sol")

	// Then: success accounting is skipped and failover remains possible.
	var failoverErr *UpstreamFailoverError
	require.Nil(t, usage)
	require.ErrorAs(t, err, &failoverErr)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestOpenAIResponsesEmptyCompletedPassthroughNonStreamingFailure(t *testing.T) {
	// Given: passthrough stream=false receives an empty terminal SSE body.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	body := task14SSEBody(
		`data: {"type":"response.created","response":{"id":"resp_empty","status":"in_progress"}}`,
		`data: {"type":"response.done","response":{"id":"resp_empty","status":"completed"}}`,
	)
	upstream := &httpUpstreamRecorder{resp: task14SSEResponseWithBody("rid-empty-pass", body)}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}

	// When: Forward uses the passthrough non-streaming path.
	result, err := svc.Forward(context.Background(), c, task14PassthroughAccount(), []byte(`{"model":"gpt-5.6-sol","stream":false,"instructions":"use tool","input":[{"type":"message","role":"user","content":"hi"}]}`))

	// Then: the empty done event is a failover candidate, not a 200 empty reply.
	var failoverErr *UpstreamFailoverError
	require.Nil(t, result)
	require.ErrorAs(t, err, &failoverErr)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestGate14StandardConvertedSSEWithVisibleDeltaAndEmptyTerminalSucceeds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}}
	body := gate14VisibleDeltaEmptyTerminalBody()

	usage, err := svc.handleSSEToJSON(resp, c, &Account{ID: 714, Platform: PlatformOpenAI}, body, "gpt-5.6-sol", "gpt-5.6-sol")

	require.NoError(t, err)
	require.NotNil(t, usage)
	require.Contains(t, rec.Body.String(), `"text":"hi"`)
}

func TestGate14PassthroughConvertedSSEWithVisibleDeltaAndEmptyTerminalSucceeds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	body := gate14VisibleDeltaEmptyTerminalBody()
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(string(body)))}}}
	account := &Account{ID: 714, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"}, Extra: map[string]any{"openai_passthrough": true}, RateMultiplier: f64p(1)}

	result, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5.6-sol","stream":false,"instructions":"hi","input":[{"type":"message","role":"user","content":"hi"}]}`))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"text":"hi"`)
}

func gate14VisibleDeltaEmptyTerminalBody() []byte {
	return []byte(strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_delta","status":"in_progress"}}`,
		`data: {"type":"response.output_text.delta","delta":"hi"}`,
		`data: {"type":"response.completed","response":{"id":"resp_delta","status":"completed","output":[]}}`,
	}, "\n"))
}

func TestOpenAIResponsesTerminalImageOnlySucceeds(t *testing.T) {
	// Given: a terminal response with image output but no text.
	body := []string{
		`data: {"type":"response.completed","response":{"id":"resp_image","status":"completed","output":[{"id":"img_1","type":"image_generation_call","result":"iVBORw0KGgo="}]}}`,
	}

	// When: the stream is handled.
	firstTokenMs, err := runTask14StreamingFrames(t, false, 0, body)

	// Then: image-only output is usable output and remains successful.
	require.NoError(t, err)
	require.NotNil(t, firstTokenMs)
}

func TestOpenAIResponsesTerminalNonRetryableFailedEventStillWritesProtocolError(t *testing.T) {
	// Given: a non-retryable terminal response.failed event.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	body := task14SSEBody(`data: {"type":"response.failed","error":{"type":"content_policy_violation","message":"content policy blocked"}}`)

	// When: the non-streaming converter handles it.
	usage, err := svc.handleSSEToJSON(task14SSEResponse("rid-policy"), c, task14OpenAIAccount(), body, "gpt-5.6-sol", "gpt-5.6-sol")

	// Then: non-retryable policy failures are returned, not retried across accounts.
	var failoverErr *UpstreamFailoverError
	require.Nil(t, usage)
	require.Error(t, err)
	require.False(t, errors.As(err, &failoverErr))
	require.True(t, c.Writer.Written())
	require.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestOpenAIResponsesTerminalPassthroughCapacityFailedEventFailsOver(t *testing.T) {
	// Given: passthrough stream=false receives the same retryable capacity event as stream=true.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	body := task14SSEBody(`data: {"type":"response.failed","error":{"type":"invalid_request_error","message":"Selected model is at capacity. Please try a different model."}}`)
	upstream := &httpUpstreamRecorder{resp: task14SSEResponseWithBody("rid-capacity-pass", body)}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}

	// When: Forward uses the passthrough non-streaming path.
	result, err := svc.Forward(context.Background(), c, task14PassthroughAccount(), []byte(`{"model":"gpt-5.6-sol","stream":false,"instructions":"use tool","input":[{"type":"message","role":"user","content":"hi"}]}`))

	// Then: the transient terminal failure remains a failover candidate.
	var failoverErr *UpstreamFailoverError
	require.Nil(t, result)
	require.ErrorAs(t, err, &failoverErr)
	require.Contains(t, string(failoverErr.ResponseBody), "Selected model is at capacity")
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func runTask14StreamingFrames(t *testing.T, passthrough bool, visibleDelay time.Duration, lines []string) (*int, error) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid-visible"}},
		Body:       pr,
	}
	go func() {
		defer func() { _ = pw.Close() }()
		for i, line := range lines {
			if visibleDelay > 0 && i == 3 {
				time.Sleep(visibleDelay)
			}
			_, _ = pw.Write([]byte(line + "\n\n"))
		}
	}()
	start := time.Now()
	if passthrough {
		result, err := svc.handleStreamingResponsePassthrough(context.Background(), resp, c, task14OpenAIAccount(), start)
		if result == nil {
			return nil, err
		}
		return result.firstTokenMs, err
	}
	result, err := svc.handleStreamingResponse(context.Background(), resp, c, task14OpenAIAccount(), start, "gpt-5.6-sol", "gpt-5.6-sol")
	if result == nil {
		return nil, err
	}
	return result.firstTokenMs, err
}

func streamModeName(passthrough bool) string {
	if passthrough {
		return "passthrough"
	}
	return "standard"
}

func task14OpenAIAccount() *Account {
	return &Account{ID: 714, Name: "task14", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1}
}

func task14PassthroughAccount() *Account {
	account := task14OpenAIAccount()
	account.Type = AccountTypeOAuth
	account.Credentials = map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"}
	account.Extra = map[string]any{"openai_passthrough": true}
	account.Status = StatusActive
	account.Schedulable = true
	account.RateMultiplier = f64p(1)
	return account
}

func task14SSEResponse(requestID string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{requestID}}}
}

func task14SSEResponseWithBody(requestID string, body []byte) *http.Response {
	resp := task14SSEResponse(requestID)
	resp.Body = io.NopCloser(strings.NewReader(string(body)))
	return resp
}

func task14SSEBody(lines ...string) []byte {
	return []byte(strings.Join(lines, "\n"))
}
