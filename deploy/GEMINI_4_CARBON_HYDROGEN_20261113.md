# Gemini 4 Carbon / Hydrogen：0.1.344

## 真实上游验证

2026-11-13，Gemini API `models.list` 已列出两个原始 ID：

- `gemini-4-carbon`
- `gemini-4-hydrogen`

账号 1842 的原生 generateContent 和 OpenAI chat/completions 均真实完成两个模型，
`modelVersion/model` 回传对应原始 ID，不把 Argon/Flash 改名冒充新型号。
1809 的 Hydrogen 也通过两个协议。未通过对应模型推理的账号不增加映射。

Carbon 在 1806/1809/1820 的免费层返回 429、对应模型配额为 0；
这是所选型号无额度，不是整 Key 失效，不禁用这些 Key 的其它模型。

## 已复现并修复

| 请求 | 上游实测 | 0.1.344 |
| --- | --- | --- |
| Carbon `reasoning_effort=max` | 400：max 非法 | 发送 high |
| Carbon `reasoning_effort=xhigh` | 400：不支持 XHIGH | 发送 high |
| Hydrogen `reasoning_effort=max` | 400：max 非法 | 发送 xhigh |
| Hydrogen `reasoning_effort=xhigh` | 200 | 原样保留，不降级 high |

Chat、Responses→Chat、Messages→Gemini 使用对应型号的规则。
原生 xhigh 被正确记录为思考等级；xhigh 使用既有高思考 90s/120s 头等待/总预算，
不新增无限重试，不截断已建立的正常 SSE，不影响 GPT Sol 的 max。
兼容 ZCode 等客户端的 `thinking` / `enable_thinking` / `reasoning` 私有字段：先归并到 `reasoning_effort`，再从发往 Google 的 JSON 删除，避免 `Unknown name` 400。
两个新模型沿用 Gemini 4 采样透传，不套用 Flash 的采样删除规则；
上游可能忽略采样，200 不代表参数影响了生成结果。

加入原生目录、管理端测试目录、前端白名单和原名映射预设。
定价使用当前官方发布口径，并独立处理 Carbon 促销截止，避免误用 Argon 的截止日期。
Hydrogen 发布价格需在 2027 年前复核；不实现 Interactions/background 长解码。

## 依据与上线门禁

- 官方发布：`https://blog.google/innovation-and-ai/technology/developers-tools/gemini-4-carbon-hydrogen-gemini-api-ai-studio/`
- Carbon：`https://aistudio.google.com/docs/models/gemini-4-carbon`
- OpenAI 兼容规则：`https://ai.google.dev/gemini-api/docs/openai`
- 定价：`https://aistudio.google.com/docs/pricing.md`

证据与凭证备份仅存 VPS root-only `/root/sub2api-gemini-20261113/`，不进入发布包。
管理 API 只合并已验证模型的映射，不改原 base URL、凭证、分组或策略。
上线后验收公网原生、Chat、Responses、SSE、真实工具多轮及 GPT/Claude 不回归。
