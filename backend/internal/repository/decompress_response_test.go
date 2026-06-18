package repository

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func TestDecompressResponseBodyZstd(t *testing.T) {
	payload := []byte(`{"ok":true}`)
	var compressed bytes.Buffer
	zw, err := zstd.NewWriter(&compressed)
	require.NoError(t, err)
	_, err = zw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	resp := &http.Response{
		Header: http.Header{
			"Content-Encoding": []string{"zstd"},
			"Content-Length":   []string{"123"},
		},
		Body:          io.NopCloser(bytes.NewReader(compressed.Bytes())),
		ContentLength: int64(compressed.Len()),
	}

	decompressResponseBody(resp)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, payload, body)
	require.Empty(t, resp.Header.Get("Content-Encoding"))
	require.Empty(t, resp.Header.Get("Content-Length"))
	require.Equal(t, int64(-1), resp.ContentLength)
	require.NoError(t, resp.Body.Close())
}
