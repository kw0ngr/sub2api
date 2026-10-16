package admin

import (
	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// JailbreakPolicyHandler 破限策略管理（Grok 分组）。
// 策略全文 JSON 存 settings 表 jailbreak_grok_policy；grok-lab 导出 →
// 此处导入 → 写入即失效缓存热更新。
type JailbreakPolicyHandler struct {
	settingService *service.SettingService
}

// NewJailbreakPolicyHandler 构造。
func NewJailbreakPolicyHandler(settingService *service.SettingService) *JailbreakPolicyHandler {
	return &JailbreakPolicyHandler{settingService: settingService}
}

// GetJailbreakPolicy GET /api/v1/admin/settings/jailbreak-grok-policy
// 返回 {policy: <策略对象>, raw: <存储的 JSON 字符串>}。无存储时回显默认配方。
func (h *JailbreakPolicyHandler) GetJailbreakPolicy(c *gin.Context) {
	p, raw, err := h.settingService.GetJailbreakGrokPolicy(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"policy": p, "raw": raw})
}

// UpdateJailbreakPolicyRequest PUT 请求体。
type UpdateJailbreakPolicyRequest struct {
	// Raw 策略 JSON 全文（grok-lab 导出格式）。空串 = 恢复默认配方。
	Raw string `json:"raw"`
}

// UpdateJailbreakPolicy PUT /api/v1/admin/settings/jailbreak-grok-policy
// 校验（正则可编译/模板占位符合法）→ 归一化合并默认 → 写入 → 缓存失效。
func (h *JailbreakPolicyHandler) UpdateJailbreakPolicy(c *gin.Context) {
	var req UpdateJailbreakPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	p, err := h.settingService.SetJailbreakGrokPolicy(c.Request.Context(), req.Raw)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"policy": p})
}

// ValidateJailbreakPolicy POST /api/v1/admin/settings/jailbreak-grok-policy/validate
// 干跑校验：不落库，返回归一化后的策略（用于 grok-lab 导出前自检）。
func (h *JailbreakPolicyHandler) ValidateJailbreakPolicy(c *gin.Context) {
	var req UpdateJailbreakPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Raw == "" {
		response.BadRequest(c, "raw is required")
		return
	}
	validated, err := h.settingService.ValidateJailbreakGrokPolicy(c.Request.Context(), req.Raw)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"policy": validated})
}

// ResetJailbreakPolicy DELETE /api/v1/admin/settings/jailbreak-grok-policy
// 清空存储，恢复内置默认配方。
func (h *JailbreakPolicyHandler) ResetJailbreakPolicy(c *gin.Context) {
	p, err := h.settingService.SetJailbreakGrokPolicy(c.Request.Context(), "")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"policy": p, "reset": true})
}
