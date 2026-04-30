package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	maxClaudeCodeFingerprintProfiles = 24
	claudeCodeFingerprintCacheTTL    = 60 * time.Second
	claudeCodeFingerprintErrorTTL    = 5 * time.Second
	claudeCodeFingerprintDBTimeout   = 5 * time.Second
	claudeCodeFingerprintObserveTTL  = 6 * time.Hour
)

type ClaudeCodeFingerprintProfile struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	Description             string `json:"description,omitempty"`
	Source                  string `json:"source"`
	AccountID               int64  `json:"account_id,omitempty"`
	AccountName             string `json:"account_name,omitempty"`
	UserAgent               string `json:"user_agent"`
	Accept                  string `json:"accept,omitempty"`
	ContentType             string `json:"content_type,omitempty"`
	AnthropicVersion        string `json:"anthropic_version,omitempty"`
	AnthropicBeta           string `json:"anthropic_beta,omitempty"`
	XApp                    string `json:"x_app,omitempty"`
	DirectBrowserAccess     string `json:"direct_browser_access,omitempty"`
	StainlessLang           string `json:"stainless_lang,omitempty"`
	StainlessPackageVersion string `json:"stainless_package_version,omitempty"`
	StainlessOS             string `json:"stainless_os,omitempty"`
	StainlessArch           string `json:"stainless_arch,omitempty"`
	StainlessRuntime        string `json:"stainless_runtime,omitempty"`
	StainlessRuntimeVersion string `json:"stainless_runtime_version,omitempty"`
	StainlessRetryCount     string `json:"stainless_retry_count,omitempty"`
	StainlessTimeout        string `json:"stainless_timeout,omitempty"`
	HelperMethod            string `json:"helper_method,omitempty"`
	CompletenessScore       int    `json:"completeness_score"`
	CreatedAt               int64  `json:"created_at"`
	UpdatedAt               int64  `json:"updated_at"`
	LastSeenAt              int64  `json:"last_seen_at"`
	SeenCount               int64  `json:"seen_count"`
}

type ClaudeCodeFingerprintLibrary struct {
	Profiles []ClaudeCodeFingerprintProfile `json:"profiles"`
	ActiveID string                         `json:"active_id"`
}

type cachedClaudeCodeFingerprintLibrary struct {
	library   ClaudeCodeFingerprintLibrary
	expiresAt int64
}

var claudeCodeFingerprintLibraryCache atomic.Value // *cachedClaudeCodeFingerprintLibrary
var claudeCodeFingerprintLibrarySF singleflight.Group
var claudeCodeFingerprintObserveMemo sync.Map // profile ID -> unix nano

func (s *SettingService) GetClaudeCodeFingerprintLibrary(ctx context.Context) (ClaudeCodeFingerprintLibrary, error) {
	return s.getClaudeCodeFingerprintLibraryCached(ctx)
}

func (s *SettingService) SetActiveClaudeCodeFingerprintProfile(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	library, err := s.getClaudeCodeFingerprintLibraryUncached(ctx)
	if err != nil {
		return err
	}
	if id != "" && !claudeCodeFingerprintProfileExists(library.Profiles, id) {
		return ErrSettingNotFound
	}
	if err := s.settingRepo.Set(ctx, SettingKeyActiveClaudeCodeFingerprintProfileID, id); err != nil {
		return err
	}
	s.invalidateClaudeCodeFingerprintLibraryCache()
	return nil
}

func (s *SettingService) DeleteClaudeCodeFingerprintProfile(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}

	library, err := s.getClaudeCodeFingerprintLibraryUncached(ctx)
	if err != nil {
		return err
	}
	next := make([]ClaudeCodeFingerprintProfile, 0, len(library.Profiles))
	for _, profile := range library.Profiles {
		if profile.ID != id {
			next = append(next, profile)
		}
	}

	payload, err := json.Marshal(next)
	if err != nil {
		return err
	}
	updates := map[string]string{
		SettingKeyClaudeCodeFingerprintProfiles: string(payload),
	}
	if library.ActiveID == id {
		updates[SettingKeyActiveClaudeCodeFingerprintProfileID] = ""
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return err
	}
	claudeCodeFingerprintObserveMemo.Delete(id)
	s.invalidateClaudeCodeFingerprintLibraryCache()
	return nil
}

func (s *SettingService) ObserveClaudeCodeFingerprint(ctx context.Context, accountID int64, accountName string, headers http.Header) error {
	if s == nil || s.settingRepo == nil || !looksLikeClaudeCodeHeaders(headers) {
		return nil
	}

	now := time.Now()
	profile := claudeCodeFingerprintProfileFromHeaders(accountID, accountName, headers, now)
	if profile.ID == "" {
		return nil
	}
	if raw, ok := claudeCodeFingerprintObserveMemo.Load(profile.ID); ok {
		if last, ok := raw.(int64); ok && now.Sub(time.Unix(0, last)) < claudeCodeFingerprintObserveTTL {
			return nil
		}
	}

	library, err := s.getClaudeCodeFingerprintLibraryUncached(ctx)
	if err != nil {
		return err
	}
	merged := false
	for i := range library.Profiles {
		if library.Profiles[i].ID != profile.ID {
			continue
		}
		merged = true
		library.Profiles[i].AccountID = accountID
		library.Profiles[i].AccountName = strings.TrimSpace(accountName)
		library.Profiles[i].UpdatedAt = now.Unix()
		library.Profiles[i].LastSeenAt = now.Unix()
		library.Profiles[i].SeenCount++
		library.Profiles[i].UserAgent = profile.UserAgent
		library.Profiles[i].Accept = profile.Accept
		library.Profiles[i].ContentType = profile.ContentType
		library.Profiles[i].AnthropicVersion = profile.AnthropicVersion
		library.Profiles[i].AnthropicBeta = profile.AnthropicBeta
		library.Profiles[i].XApp = profile.XApp
		library.Profiles[i].DirectBrowserAccess = profile.DirectBrowserAccess
		library.Profiles[i].StainlessLang = profile.StainlessLang
		library.Profiles[i].StainlessPackageVersion = profile.StainlessPackageVersion
		library.Profiles[i].StainlessOS = profile.StainlessOS
		library.Profiles[i].StainlessArch = profile.StainlessArch
		library.Profiles[i].StainlessRuntime = profile.StainlessRuntime
		library.Profiles[i].StainlessRuntimeVersion = profile.StainlessRuntimeVersion
		library.Profiles[i].StainlessRetryCount = profile.StainlessRetryCount
		library.Profiles[i].StainlessTimeout = profile.StainlessTimeout
		library.Profiles[i].HelperMethod = profile.HelperMethod
		library.Profiles[i].CompletenessScore = profile.CompletenessScore
		break
	}
	if !merged {
		library.Profiles = append(library.Profiles, profile)
	}
	sortClaudeCodeFingerprintProfiles(library.Profiles)
	if len(library.Profiles) > maxClaudeCodeFingerprintProfiles {
		library.Profiles = library.Profiles[:maxClaudeCodeFingerprintProfiles]
	}

	payload, err := json.Marshal(library.Profiles)
	if err != nil {
		return err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyClaudeCodeFingerprintProfiles, string(payload)); err != nil {
		return err
	}
	claudeCodeFingerprintObserveMemo.Store(profile.ID, now.UnixNano())
	s.invalidateClaudeCodeFingerprintLibraryCache()
	return nil
}

func (s *SettingService) ApplyActiveClaudeCodeFingerprint(ctx context.Context, base *Fingerprint) (*Fingerprint, bool) {
	if s == nil || s.settingRepo == nil {
		return nil, false
	}
	library, err := s.getClaudeCodeFingerprintLibraryCached(ctx)
	if err != nil || strings.TrimSpace(library.ActiveID) == "" {
		return nil, false
	}
	for _, profile := range library.Profiles {
		if profile.ID == library.ActiveID {
			return profile.toFingerprint(base), true
		}
	}
	return nil, false
}

func (s *SettingService) getClaudeCodeFingerprintLibraryCached(ctx context.Context) (ClaudeCodeFingerprintLibrary, error) {
	now := time.Now()
	if cached, ok := claudeCodeFingerprintLibraryCache.Load().(*cachedClaudeCodeFingerprintLibrary); ok && cached.expiresAt > now.UnixNano() {
		return cached.library, nil
	}

	val, err, _ := claudeCodeFingerprintLibrarySF.Do("library", func() (any, error) {
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), claudeCodeFingerprintDBTimeout)
		defer cancel()
		library, err := s.getClaudeCodeFingerprintLibraryUncached(dbCtx)
		ttl := claudeCodeFingerprintCacheTTL
		if err != nil {
			ttl = claudeCodeFingerprintErrorTTL
		}
		claudeCodeFingerprintLibraryCache.Store(&cachedClaudeCodeFingerprintLibrary{
			library:   library,
			expiresAt: time.Now().Add(ttl).UnixNano(),
		})
		return library, err
	})
	if err != nil {
		return ClaudeCodeFingerprintLibrary{}, err
	}
	if library, ok := val.(ClaudeCodeFingerprintLibrary); ok {
		return library, nil
	}
	return ClaudeCodeFingerprintLibrary{}, nil
}

func (s *SettingService) getClaudeCodeFingerprintLibraryUncached(ctx context.Context) (ClaudeCodeFingerprintLibrary, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyClaudeCodeFingerprintProfiles,
		SettingKeyActiveClaudeCodeFingerprintProfileID,
	})
	if err != nil {
		return ClaudeCodeFingerprintLibrary{}, err
	}
	profiles := parseClaudeCodeFingerprintProfiles(values[SettingKeyClaudeCodeFingerprintProfiles])
	sortClaudeCodeFingerprintProfiles(profiles)
	activeID := strings.TrimSpace(values[SettingKeyActiveClaudeCodeFingerprintProfileID])
	if activeID != "" && !claudeCodeFingerprintProfileExists(profiles, activeID) {
		activeID = ""
	}
	return ClaudeCodeFingerprintLibrary{Profiles: profiles, ActiveID: activeID}, nil
}

func (s *SettingService) invalidateClaudeCodeFingerprintLibraryCache() {
	claudeCodeFingerprintLibraryCache.Store(&cachedClaudeCodeFingerprintLibrary{
		expiresAt: time.Now().Add(-time.Second).UnixNano(),
	})
}

func parseClaudeCodeFingerprintProfiles(raw string) []ClaudeCodeFingerprintProfile {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var profiles []ClaudeCodeFingerprintProfile
	if err := json.Unmarshal([]byte(raw), &profiles); err != nil {
		return nil
	}
	for i := range profiles {
		normalizeClaudeCodeFingerprintProfile(&profiles[i])
	}
	return profiles
}

func normalizeClaudeCodeFingerprintProfile(profile *ClaudeCodeFingerprintProfile) {
	if profile == nil {
		return
	}
	if profile.Accept == "" {
		profile.Accept = defaultFingerprint.Accept
	}
	if profile.ContentType == "" {
		profile.ContentType = defaultFingerprint.ContentType
	}
	if profile.AnthropicVersion == "" {
		profile.AnthropicVersion = defaultFingerprint.AnthropicVersion
	}
	if profile.XApp == "" {
		profile.XApp = defaultFingerprint.XApp
	}
	if profile.DirectBrowserAccess == "" {
		profile.DirectBrowserAccess = defaultFingerprint.DirectBrowserAccess
	}
	if profile.StainlessRetryCount == "" {
		profile.StainlessRetryCount = defaultFingerprint.StainlessRetryCount
	}
	if profile.StainlessTimeout == "" {
		profile.StainlessTimeout = defaultFingerprint.StainlessTimeout
	}
	fp := profile.toFingerprint(nil)
	profile.CompletenessScore = claudeCodeFingerprintCompletenessScore(*fp)
}

func claudeCodeFingerprintProfileFromHeaders(accountID int64, accountName string, headers http.Header, now time.Time) ClaudeCodeFingerprintProfile {
	fp := Fingerprint{
		UserAgent:               strings.TrimSpace(headers.Get("User-Agent")),
		Accept:                  strings.TrimSpace(headers.Get("Accept")),
		ContentType:             strings.TrimSpace(headers.Get("Content-Type")),
		AnthropicVersion:        strings.TrimSpace(headers.Get("anthropic-version")),
		AnthropicBeta:           strings.TrimSpace(headers.Get("anthropic-beta")),
		XApp:                    strings.TrimSpace(headers.Get("X-App")),
		DirectBrowserAccess:     strings.TrimSpace(headers.Get("Anthropic-Dangerous-Direct-Browser-Access")),
		StainlessLang:           getHeaderOrDefault(headers, "X-Stainless-Lang", defaultFingerprint.StainlessLang),
		StainlessPackageVersion: getHeaderOrDefault(headers, "X-Stainless-Package-Version", defaultFingerprint.StainlessPackageVersion),
		StainlessOS:             getHeaderOrDefault(headers, "X-Stainless-OS", defaultFingerprint.StainlessOS),
		StainlessArch:           getHeaderOrDefault(headers, "X-Stainless-Arch", defaultFingerprint.StainlessArch),
		StainlessRuntime:        getHeaderOrDefault(headers, "X-Stainless-Runtime", defaultFingerprint.StainlessRuntime),
		StainlessRuntimeVersion: getHeaderOrDefault(headers, "X-Stainless-Runtime-Version", defaultFingerprint.StainlessRuntimeVersion),
		StainlessRetryCount:     getHeaderOrDefault(headers, "X-Stainless-Retry-Count", defaultFingerprint.StainlessRetryCount),
		StainlessTimeout:        getHeaderOrDefault(headers, "X-Stainless-Timeout", defaultFingerprint.StainlessTimeout),
	}
	if fp.Accept == "" {
		fp.Accept = defaultFingerprint.Accept
	}
	if fp.ContentType == "" {
		fp.ContentType = defaultFingerprint.ContentType
	}
	if fp.AnthropicVersion == "" {
		fp.AnthropicVersion = defaultFingerprint.AnthropicVersion
	}
	if fp.XApp == "" {
		fp.XApp = defaultFingerprint.XApp
	}
	if fp.DirectBrowserAccess == "" {
		fp.DirectBrowserAccess = defaultFingerprint.DirectBrowserAccess
	}
	id := claudeCodeFingerprintProfileID(fp)
	version := ExtractCLIVersion(fp.UserAgent)
	nameParts := []string{"Claude Code"}
	if version != "" {
		nameParts = append(nameParts, version)
	}
	if fp.StainlessOS != "" || fp.StainlessArch != "" {
		nameParts = append(nameParts, "·", strings.Trim(strings.Join([]string{fp.StainlessOS, fp.StainlessArch}, "/"), "/"))
	}
	description := "自动捕获自真实 Claude Code 请求"
	if accountName = strings.TrimSpace(accountName); accountName != "" {
		description += "：" + accountName
	}
	return ClaudeCodeFingerprintProfile{
		ID:                      id,
		Name:                    strings.Join(nameParts, " "),
		Description:             description,
		Source:                  "auto_capture",
		AccountID:               accountID,
		AccountName:             accountName,
		UserAgent:               fp.UserAgent,
		Accept:                  fp.Accept,
		ContentType:             fp.ContentType,
		AnthropicVersion:        fp.AnthropicVersion,
		AnthropicBeta:           fp.AnthropicBeta,
		XApp:                    fp.XApp,
		DirectBrowserAccess:     fp.DirectBrowserAccess,
		StainlessLang:           fp.StainlessLang,
		StainlessPackageVersion: fp.StainlessPackageVersion,
		StainlessOS:             fp.StainlessOS,
		StainlessArch:           fp.StainlessArch,
		StainlessRuntime:        fp.StainlessRuntime,
		StainlessRuntimeVersion: fp.StainlessRuntimeVersion,
		StainlessRetryCount:     fp.StainlessRetryCount,
		StainlessTimeout:        fp.StainlessTimeout,
		HelperMethod:            strings.TrimSpace(headers.Get("X-Stainless-Helper-Method")),
		CompletenessScore:       claudeCodeFingerprintCompletenessScore(fp),
		CreatedAt:               now.Unix(),
		UpdatedAt:               now.Unix(),
		LastSeenAt:              now.Unix(),
		SeenCount:               1,
	}
}

func (p ClaudeCodeFingerprintProfile) toFingerprint(base *Fingerprint) *Fingerprint {
	out := &Fingerprint{}
	if base != nil {
		cp := *base
		out = &cp
	}
	if p.UserAgent != "" {
		out.UserAgent = p.UserAgent
	}
	if p.Accept != "" {
		out.Accept = p.Accept
	}
	if p.ContentType != "" {
		out.ContentType = p.ContentType
	}
	if p.AnthropicVersion != "" {
		out.AnthropicVersion = p.AnthropicVersion
	}
	if p.AnthropicBeta != "" {
		out.AnthropicBeta = p.AnthropicBeta
	}
	if p.XApp != "" {
		out.XApp = p.XApp
	}
	if p.DirectBrowserAccess != "" {
		out.DirectBrowserAccess = p.DirectBrowserAccess
	}
	if p.StainlessLang != "" {
		out.StainlessLang = p.StainlessLang
	}
	if p.StainlessPackageVersion != "" {
		out.StainlessPackageVersion = p.StainlessPackageVersion
	}
	if p.StainlessOS != "" {
		out.StainlessOS = p.StainlessOS
	}
	if p.StainlessArch != "" {
		out.StainlessArch = p.StainlessArch
	}
	if p.StainlessRuntime != "" {
		out.StainlessRuntime = p.StainlessRuntime
	}
	if p.StainlessRuntimeVersion != "" {
		out.StainlessRuntimeVersion = p.StainlessRuntimeVersion
	}
	if p.StainlessRetryCount != "" {
		out.StainlessRetryCount = p.StainlessRetryCount
	}
	if p.StainlessTimeout != "" {
		out.StainlessTimeout = p.StainlessTimeout
	}
	return out
}

func claudeCodeFingerprintProfileID(fp Fingerprint) string {
	parts := []string{
		fp.UserAgent,
		fp.Accept,
		fp.ContentType,
		fp.AnthropicVersion,
		fp.XApp,
		fp.DirectBrowserAccess,
		fp.StainlessLang,
		fp.StainlessPackageVersion,
		fp.StainlessOS,
		fp.StainlessArch,
		fp.StainlessRuntime,
		fp.StainlessRuntimeVersion,
		fp.StainlessRetryCount,
		fp.StainlessTimeout,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:8])
}

func claudeCodeFingerprintCompletenessScore(fp Fingerprint) int {
	fields := []string{
		fp.UserAgent,
		fp.Accept,
		fp.ContentType,
		fp.AnthropicVersion,
		fp.XApp,
		fp.DirectBrowserAccess,
		fp.StainlessLang,
		fp.StainlessPackageVersion,
		fp.StainlessOS,
		fp.StainlessArch,
		fp.StainlessRuntime,
		fp.StainlessRuntimeVersion,
		fp.StainlessRetryCount,
		fp.StainlessTimeout,
	}
	if len(fields) == 0 {
		return 0
	}
	present := 0
	for _, field := range fields {
		if strings.TrimSpace(field) != "" {
			present++
		}
	}
	return present * 100 / len(fields)
}

func looksLikeClaudeCodeHeaders(headers http.Header) bool {
	if headers == nil {
		return false
	}
	return ExtractCLIVersion(strings.TrimSpace(headers.Get("User-Agent"))) != ""
}

func claudeCodeFingerprintProfileExists(profiles []ClaudeCodeFingerprintProfile, id string) bool {
	for _, profile := range profiles {
		if profile.ID == id {
			return true
		}
	}
	return false
}

func sortClaudeCodeFingerprintProfiles(profiles []ClaudeCodeFingerprintProfile) {
	sort.SliceStable(profiles, func(i, j int) bool {
		if profiles[i].LastSeenAt == profiles[j].LastSeenAt {
			return profiles[i].CreatedAt > profiles[j].CreatedAt
		}
		return profiles[i].LastSeenAt > profiles[j].LastSeenAt
	})
}
