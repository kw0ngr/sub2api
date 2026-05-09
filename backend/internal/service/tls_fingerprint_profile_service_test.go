package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/stretchr/testify/require"
)

type tlsProfileRepoStub struct {
	profiles []*model.TLSFingerprintProfile
}

func (r *tlsProfileRepoStub) List(context.Context) ([]*model.TLSFingerprintProfile, error) {
	return r.profiles, nil
}

func (r *tlsProfileRepoStub) GetByID(_ context.Context, id int64) (*model.TLSFingerprintProfile, error) {
	for _, p := range r.profiles {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}

func (r *tlsProfileRepoStub) Create(_ context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	return profile, nil
}

func (r *tlsProfileRepoStub) Update(_ context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	return profile, nil
}

func (r *tlsProfileRepoStub) Delete(context.Context, int64) error {
	return nil
}

func TestTLSFingerprintProfileService_ListIncludesBuiltIns(t *testing.T) {
	svc := NewTLSFingerprintProfileService(&tlsProfileRepoStub{}, nil)

	profiles, err := svc.List(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, profiles)
	require.True(t, profiles[0].Builtin)
	require.Equal(t, tlsBuiltinClaudeCodeNode24ID, profiles[0].ID)
	require.NotEmpty(t, profiles[0].CipherSuites)
	require.NotEmpty(t, profiles[0].Extensions)
	require.Equal(t, "44f88fca027f27bab4bb08d4af15f23e", profiles[0].JA3Hash)
}

func TestTLSFingerprintProfileService_ResolveDefaultUsesBuiltInNode24(t *testing.T) {
	svc := NewTLSFingerprintProfileService(&tlsProfileRepoStub{}, nil)
	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			"enable_tls_fingerprint": true,
		},
	}

	profile := svc.ResolveTLSProfile(account)
	require.NotNil(t, profile)
	require.Equal(t, "Claude Code / Node.js 24.x", profile.Name)
	require.NotEmpty(t, profile.CipherSuites)
	require.NotEmpty(t, profile.Extensions)
}

func TestTLSFingerprintProfileService_ResolveBuiltInByID(t *testing.T) {
	svc := NewTLSFingerprintProfileService(&tlsProfileRepoStub{}, nil)
	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": float64(tlsBuiltinNode22LinuxX64ID),
		},
	}

	profile := svc.ResolveTLSProfile(account)
	require.NotNil(t, profile)
	require.Equal(t, "Node.js 22.x / Linux x64", profile.Name)
	require.Equal(t, []uint16{0, 11, 10, 35, 16, 22, 23, 13, 43, 45, 51}, profile.Extensions)
}
