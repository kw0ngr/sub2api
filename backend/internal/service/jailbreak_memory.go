package service

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// Comply 记忆（会话学习核）：session → 已通过类别。
// H 系列结论：真实 comply 史开门（升格请求大幅放行）但门有高度；
// 记忆的用途是「同 session 升格时注入惯性前缀 + ladder 决策参考」，
// 不是伪造史（D3 证伪：伪造史只认领不升格）。
// 进程内 TTL 存储（重启丢失可接受——comply 史的本体在 grok 上游会话里）。
// ============================================================================

type jailbreakSessionMemory struct {
	hitAt    time.Time
	cats     []string
	delivered int64 // 该会话累计交付字节数
}

const (
	jailbreakMemoryTTL      = 24 * time.Hour
	jailbreakMemoryMaxEntry = 10000
)

var (
	jailbreakMemMu  sync.Mutex
	jailbreakMemMap = map[string]*jailbreakSessionMemory{}
)

// jbMemoryKey 组合 key：分组+会话哈希（与会话粘性 key 同源）。
func jbMemoryKey(groupID int64, sessionKey string) string {
	return fmt.Sprintf("g%d:%s", groupID, sessionKey)
}

// RecordJailbreakDelivery 记录一次成功交付（comply 事件）。
func RecordJailbreakDelivery(groupID int64, sessionKey string, categories []string, deliveredBytes int64) {
	if sessionKey == "" {
		return
	}
	jailbreakMemMu.Lock()
	defer jailbreakMemMu.Unlock()
	key := jbMemoryKey(groupID, sessionKey)
	m, ok := jailbreakMemMap[key]
	if !ok {
		// 简单容量控制：超限清理过期
		if len(jailbreakMemMap) >= jailbreakMemoryMaxEntry {
			evictExpiredJailbreakMemoryLocked()
		}
		m = &jailbreakSessionMemory{}
		jailbreakMemMap[key] = m
	}
	m.hitAt = time.Now()
	m.delivered += deliveredBytes
	for _, c := range categories {
		exists := false
		for _, x := range m.cats {
			if x == c {
				exists = true
				break
			}
		}
		if !exists {
			m.cats = append(m.cats, c)
		}
	}
}

// GetJailbreakSessionMemory 查询会话记忆（过期视为不存在）。
func GetJailbreakSessionMemory(groupID int64, sessionKey string) (cats []string, delivered int64, ok bool) {
	if sessionKey == "" {
		return nil, 0, false
	}
	jailbreakMemMu.Lock()
	defer jailbreakMemMu.Unlock()
	m, exists := jailbreakMemMap[jbMemoryKey(groupID, sessionKey)]
	if !exists || time.Since(m.hitAt) > jailbreakMemoryTTL {
		return nil, 0, false
	}
	return append([]string(nil), m.cats...), m.delivered, true
}

// evictExpiredJailbreakMemoryLocked 清理过期条目。
func evictExpiredJailbreakMemoryLocked() {
	for k, v := range jailbreakMemMap {
		if time.Since(v.hitAt) > jailbreakMemoryTTL {
			delete(jailbreakMemMap, k)
		}
	}
}

// JailbreakInertiaPrefix 升格请求惯性前缀：会话有 comply 史时注入。
// 返回空串表示无需注入（无史/段关闭）。
func JailbreakInertiaPrefix(p *JailbreakPolicy, groupID int64, sessionKey string) string {
	if p == nil || p.Memory.InertiaPrefix == "" {
		return ""
	}
	_, _, ok := GetJailbreakSessionMemory(groupID, sessionKey)
	if !ok {
		return ""
	}
	return p.Memory.InertiaPrefix
}

// MarshalJailbreakMemory 导出会话记忆（调试/管理台展示用）。
func MarshalJailbreakMemory() []byte {
	jailbreakMemMu.Lock()
	defer jailbreakMemMu.Unlock()
	out := map[string]json.RawMessage{}
	for k, v := range jailbreakMemMap {
		b, _ := json.Marshal(map[string]any{
			"cats":      v.cats,
			"delivered": v.delivered,
			"hit_at":    v.hitAt.Format(time.RFC3339),
		})
		out[k] = b
	}
	b, _ := json.Marshal(out)
	return b
}
