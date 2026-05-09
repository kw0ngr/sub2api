package service

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

const (
	tlsBuiltinClaudeCodeNode24ID int64 = -1001
	tlsBuiltinNode22LinuxX64ID   int64 = -1002
)

var tlsBuiltinProfileUpdatedAt = time.Unix(0, 0).UTC()

func builtInTLSFingerprintProfiles() []*model.TLSFingerprintProfile {
	node24 := tlsfingerprint.DefaultNode24Profile()
	return []*model.TLSFingerprintProfile{
		modelFromBuiltInTLSProfile(
			tlsBuiltinClaudeCodeNode24ID,
			"claude_code_node24",
			node24,
			"内置：Claude Code / Node.js 24.x ClientHello，优先用于 Claude Code 2.x / Node 24.x 指纹绑定。",
			"44f88fca027f27bab4bb08d4af15f23e",
			"t13d1714h1_5b57614c22b0_7baf387fc6ff",
		),
		modelFromBuiltInTLSProfile(
			tlsBuiltinNode22LinuxX64ID,
			"node22_linux_x64",
			&tlsfingerprint.Profile{
				Name:         "Node.js 22.x / Linux x64",
				EnableGREASE: false,
				CipherSuites: []uint16{4866, 4867, 4865, 49199, 49195, 49200, 49196, 158, 49191, 103, 49192, 107, 163, 159, 52393, 52392, 52394, 49327, 49325, 49315, 49311, 49245, 49249, 49239, 49235, 162, 49326, 49324, 49314, 49310, 49244, 49248, 49238, 49234, 49188, 106, 49187, 64, 49162, 49172, 57, 56, 49161, 49171, 51, 50, 157, 49313, 49309, 49233, 156, 49312, 49308, 49232, 61, 60, 53, 47, 255},
				Curves:       []uint16{29, 23, 30, 25, 24, 256, 257, 258, 259, 260},
				PointFormats: []uint16{0, 1, 2},
				ALPNProtocols: []string{
					"http/1.1",
				},
				SupportedVersions: []uint16{0x0304, 0x0303},
				KeyShareGroups:    []uint16{29},
				PSKModes:          []uint16{1},
				Extensions:        []uint16{0, 11, 10, 35, 16, 22, 23, 13, 43, 45, 51},
			},
			"内置：Node.js 22.x / Linux x64 显式 ClientHello，作为 Node 24 不匹配时的备用模板。",
			"",
			"",
		),
	}
}

func builtInTLSFingerprintProfileByID(id int64) (*model.TLSFingerprintProfile, bool) {
	for _, p := range builtInTLSFingerprintProfiles() {
		if p.ID == id {
			return p, true
		}
	}
	return nil, false
}

func builtInRuntimeTLSProfileByID(id int64) (*tlsfingerprint.Profile, bool) {
	p, ok := builtInTLSFingerprintProfileByID(id)
	if !ok {
		return nil, false
	}
	return p.ToTLSProfile(), true
}

func modelFromBuiltInTLSProfile(id int64, key string, profile *tlsfingerprint.Profile, description string, ja3Hash string, ja4 string) *model.TLSFingerprintProfile {
	desc := description
	return &model.TLSFingerprintProfile{
		ID:                  id,
		Name:                profile.Name,
		Description:         &desc,
		Builtin:             true,
		BuiltinKey:          key,
		JA3Hash:             ja3Hash,
		JA4:                 ja4,
		EnableGREASE:        profile.EnableGREASE,
		CipherSuites:        cloneUint16Slice(profile.CipherSuites),
		Curves:              cloneUint16Slice(profile.Curves),
		PointFormats:        cloneUint16Slice(profile.PointFormats),
		SignatureAlgorithms: cloneUint16Slice(profile.SignatureAlgorithms),
		ALPNProtocols:       cloneTLSProfileStrings(profile.ALPNProtocols),
		SupportedVersions:   cloneUint16Slice(profile.SupportedVersions),
		KeyShareGroups:      cloneUint16Slice(profile.KeyShareGroups),
		PSKModes:            cloneUint16Slice(profile.PSKModes),
		Extensions:          cloneUint16Slice(profile.Extensions),
		CreatedAt:           tlsBuiltinProfileUpdatedAt,
		UpdatedAt:           tlsBuiltinProfileUpdatedAt,
	}
}

func cloneUint16Slice(in []uint16) []uint16 {
	if len(in) == 0 {
		return []uint16{}
	}
	out := make([]uint16, len(in))
	copy(out, in)
	return out
}

func cloneTLSProfileStrings(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
