package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const usageProjectLabelMaxRunes = 120

// NormalizeUsageProjectLabel trims and bounds a client-provided project label.
func NormalizeUsageProjectLabel(project string) string {
	project = strings.TrimSpace(project)
	if project == "" {
		return ""
	}
	runes := []rune(project)
	if len(runes) > usageProjectLabelMaxRunes {
		project = string(runes[:usageProjectLabelMaxRunes])
	}
	return project
}

// ResolveUsageProjectIdentity returns a stable compact key and readable label.
// The key follows TokenArena's HMAC-SHA256/16-hex pattern, while label remains
// readable for small internal teams that explicitly submit a project name.
func ResolveUsageProjectIdentity(project string, salt string) (string, string) {
	label := NormalizeUsageProjectLabel(project)
	if label == "" {
		return "", ""
	}
	salt = strings.TrimSpace(salt)
	if salt == "" {
		salt = "sub2api-usage-project"
	}
	mac := hmac.New(sha256.New, []byte(salt))
	_, _ = mac.Write([]byte(label))
	key := hex.EncodeToString(mac.Sum(nil))
	if len(key) > 16 {
		key = key[:16]
	}
	return key, label
}

func usageProjectSaltFromConfig(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.JWT.Secret)
}
