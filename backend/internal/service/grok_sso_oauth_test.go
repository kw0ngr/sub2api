package service

import (
	"strings"
	"testing"
)

func TestStripGrokSSOPrefix(t *testing.T) {
	t.Parallel()
	if got := stripGrokSSOPrefix("sso=abc"); got != "abc" {
		t.Fatalf("got %q", got)
	}
	if got := stripGrokSSOPrefix("sso=abc; sso-rw=abc"); got != "abc" {
		t.Fatalf("cookie got %q", got)
	}
	if got := stripGrokSSOPrefix("foo=1; sso=xyz; bar=2"); got != "xyz" {
		t.Fatalf("mid cookie got %q", got)
	}
}

func TestParseGrokHiddenInputs(t *testing.T) {
	t.Parallel()
	html := `<form action="https://auth.x.ai/oauth2/authorize" method="POST">
<input type="hidden" name="client_id" value="cid"/>
<input type="hidden" name="state" value="st"/>
</form>`
	fields := parseGrokHiddenInputs(html)
	if fields["client_id"] != "cid" || fields["state"] != "st" {
		t.Fatalf("fields=%v", fields)
	}
	if !strings.Contains(html, "authorize") {
		t.Fatal("sanity")
	}
}

func TestDetectLocalCallback(t *testing.T) {
	t.Parallel()
	if !isLocalCallback("http://127.0.0.1:56121/callback?code=x") {
		t.Fatal("expected local")
	}
	if isLocalCallback("https://accounts.x.ai/oauth2/consent") {
		t.Fatal("not local")
	}
}
