package service

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCodexTurnMetadataRewritePreservesHeaderSafeJSON(t *testing.T) {
	ids := &codexFingerprintIDs{
		mode:           codexFingerprintSession,
		installationID: "gateway-install",
		sessionID:      "gateway-session",
		threadID:       "gateway-thread",
		turnID:         "gateway-turn",
		windowID:       "gateway-window",
	}
	rewriters := map[string]func(*testing.T, string) string{
		"header": func(t *testing.T, raw string) string {
			h := make(http.Header)
			h.Set(openAIWSTurnMetadataHeader, raw)
			applyCodexFingerprintHeaders(h, ids)
			return h.Get(openAIWSTurnMetadataHeader)
		},
		"body_map": func(t *testing.T, raw string) string {
			cm := map[string]any{openAIWSTurnMetadataHeader: raw}
			require.True(t, applyCodexFingerprintClientMetadata(map[string]any{"client_metadata": cm}, ids))
			return cm[openAIWSTurnMetadataHeader].(string)
		},
		"body_raw": func(t *testing.T, raw string) string {
			body, err := json.Marshal(map[string]any{
				"client_metadata": map[string]any{openAIWSTurnMetadataHeader: raw},
			})
			require.NoError(t, err)
			updated, changed, err := applyCodexFingerprintClientMetadataRaw(body, ids)
			require.NoError(t, err)
			require.True(t, changed)
			return gjson.GetBytes(updated, "client_metadata."+openAIWSTurnMetadataHeader).String()
		},
	}
	cases := map[string]string{
		"ascii":    `{"installation_id":"client-install","workspace":"C:\\work\\ascii","literal":"\\u4e2d"}`,
		"unicode":  `{"installation_id":"client-install","workspaces":{"C:\\work\\中文🚀":{"label":"café"}}}`,
		"escaped":  `{"installation_id":"client-install","workspaces":{"C:\\work\\\u4e2d\u6587\ud83d\ude80":{"label":"caf\u00e9"}}}`,
		"controls": `{"installation_id":"client-install","workspace":"中文🚀","literal":"\\u4e2d","quote":"\"\\","controls":"\b\f\n\r\t\u0000\u007f","timestamp":1789302780858,"enabled":true}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			var original map[string]any
			require.NoError(t, json.Unmarshal([]byte(raw), &original))
			for path, rewrite := range rewriters {
				t.Run(path, func(t *testing.T) {
					updated := rewrite(t, raw)
					var decoded map[string]any
					require.NoError(t, json.Unmarshal([]byte(updated), &decoded))
					require.Equal(t, ids.installationID, decoded["installation_id"])
					for key, value := range original {
						if key != "installation_id" {
							require.Equal(t, value, decoded[key], key)
						}
					}
					for _, b := range []byte(updated) {
						require.True(t, b >= 0x20 && b < 0x7f, "non-printable ASCII header byte: 0x%02x", b)
					}
				})
			}
		})
	}
}
