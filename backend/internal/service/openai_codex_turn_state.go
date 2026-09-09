package service

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	openAICodexTurnStateHeader           = "x-codex-turn-state"
	openAICodexTurnStateOriginMaxEntries = 1024
)

type openAICodexTurnStateOrigin struct {
	accountID        int64
	chatgptAccountID string
	expiresAt        time.Time
}

func openAICodexTurnStateSeed(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	sessionID := extractClientSessionID(c.Request.Header)
	if sessionID == "" {
		return ""
	}
	return strconv.FormatInt(getAPIKeyIDFromContext(c), 10) + "\x00" + sessionID
}

func (s *OpenAIGatewayService) relayOpenAICodexTurnState(c *gin.Context, account *Account, upstream http.Header) {
	if c == nil || c.Writer == nil {
		return
	}
	key := http.CanonicalHeaderKey(openAICodexTurnStateHeader)
	state := extractOpenAICodexTurnState(upstream)
	if state == "" {
		c.Writer.Header().Del(key)
		return
	}
	c.Writer.Header().Set(key, state)
	s.noteOpenAICodexTurnStateProvenance(c, account)
}

func extractOpenAICodexTurnState(upstream http.Header) string {
	if upstream == nil {
		return ""
	}
	return strings.TrimSpace(upstream.Get(openAICodexTurnStateHeader))
}

func (s *OpenAIGatewayService) noteOpenAICodexTurnStateProvenance(c *gin.Context, account *Account) {
	if s == nil || account == nil || account.ID <= 0 {
		return
	}
	seed := openAICodexTurnStateSeed(c)
	if seed == "" {
		return
	}
	now := time.Now()
	s.openaiCodexTurnStateMu.Lock()
	defer s.openaiCodexTurnStateMu.Unlock()
	if s.openaiCodexTurnStateOrigins == nil {
		s.openaiCodexTurnStateOrigins = make(map[string]openAICodexTurnStateOrigin)
	}
	s.openaiCodexTurnStateWrites++
	s.openaiCodexTurnStateOrigins[seed] = openAICodexTurnStateOrigin{
		accountID:        account.ID,
		chatgptAccountID: account.GetChatGPTAccountID(),
		expiresAt:        now.Add(s.openAIWSSessionStickyTTL()),
	}
	if s.openaiCodexTurnStateWrites%256 == 0 {
		s.sweepOpenAICodexTurnStateOriginsLocked(now)
	}
	s.trimOpenAICodexTurnStateOriginsLocked(now)
}

func (s *OpenAIGatewayService) guardOpenAICodexTurnStateEcho(c *gin.Context, account *Account, h http.Header) {
	if s == nil || h == nil || account == nil || strings.TrimSpace(h.Get(openAICodexTurnStateHeader)) == "" {
		return
	}
	seed := openAICodexTurnStateSeed(c)
	if seed == "" {
		return
	}
	now := time.Now()
	s.openaiCodexTurnStateMu.Lock()
	origin, ok := s.openaiCodexTurnStateOrigins[seed]
	if ok && !origin.expiresAt.IsZero() && now.After(origin.expiresAt) {
		delete(s.openaiCodexTurnStateOrigins, seed)
		ok = false
	}
	strip := ok && !openAICodexTurnStateOriginMatchesAccount(origin, account)
	s.openaiCodexTurnStateMu.Unlock()
	if strip {
		h.Del(openAICodexTurnStateHeader)
	}
}

func openAICodexTurnStateOriginMatchesAccount(origin openAICodexTurnStateOrigin, account *Account) bool {
	if account == nil || origin.accountID != account.ID {
		return false
	}
	currentChatGPTAccountID := account.GetChatGPTAccountID()
	if origin.chatgptAccountID == "" && currentChatGPTAccountID == "" {
		return true
	}
	return origin.chatgptAccountID == currentChatGPTAccountID
}

func (s *OpenAIGatewayService) sweepOpenAICodexTurnStateOriginsLocked(now time.Time) {
	for key, origin := range s.openaiCodexTurnStateOrigins {
		if !origin.expiresAt.IsZero() && now.After(origin.expiresAt) {
			delete(s.openaiCodexTurnStateOrigins, key)
		}
	}
}

func (s *OpenAIGatewayService) trimOpenAICodexTurnStateOriginsLocked(now time.Time) {
	if len(s.openaiCodexTurnStateOrigins) <= openAICodexTurnStateOriginMaxEntries {
		return
	}
	s.sweepOpenAICodexTurnStateOriginsLocked(now)
	for key := range s.openaiCodexTurnStateOrigins {
		if len(s.openaiCodexTurnStateOrigins) <= openAICodexTurnStateOriginMaxEntries {
			return
		}
		delete(s.openaiCodexTurnStateOrigins, key)
	}
}
