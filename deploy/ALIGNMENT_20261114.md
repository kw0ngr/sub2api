# 0.1.345：多代理官方对齐与模型池复核

## 基线

- 本地/生产起点：`da734e459`，正式版 0.1.344。
- 本轮重新 fetch 官方 `Wei-Shaw/sub2api`，main 仍为 `530c5e3a4` / 0.2.21。
- 官方没有新的 main 提交；本轮补齐尚未移植的独立修复，不以整库 merge 替换本分支。
- 保留 Grok CLI/策略链、Gemini 原生/OpenAI 兼容和签名缓存、GLM 两协议、现有配置/分组/已执行迁移及 `.omo/`。

## 已引入

| 范围 | 官方参考/现场证据 | 最小修复 |
| --- | --- | --- |
| GPT/Codex | `763443abf` | 指纹改写后 turn metadata 使用 HTTP-header-safe JSON，保留中文路径和 emoji 的转义语义。 |
| GPT 工具续链 | `4d4ba64bf` | 清除重放 message 的非法 item ID，保留合法 msg ID、工具 call_id 和调用/结果配对。 |
| Claude Responses | `0952ce341` | `response.function_call_arguments.done.arguments` 保留前序增量完整参数，不再只在最终 item 中出现。 |
| 通用 429 | `4b904c887` | 缺失或非法重置时间时复用已有可配置 fallback，不再硬编码停调 5 分钟。Grok 内容拒绝、Build 噪音及 API Key 专属策略不变。 |
| GLM | 账号 1784 的 `1308` 五小时上限错误 | 与 `1310` 一样按正文真实重置时刻处理；使用 UTC+08、与 Retry-After 取较晚值，不接受过期/非法日期。 |
| 跨平台计费取消 | GPT/Gemini 日志出现 `get subscription: context canceled` 后返回计费 503 | 客户端请求 context 已结束则返回 context 错误、映射 499，不累计计费熔断失败；半开探测仅归还本轮自己的许可。服务内部 3 秒超时仍是 503，不能掩盖真实数据库故障。 |

## 生产复核

现场备份和含凭证证据仅保留 VPS root-only：
`/root/sub2api-align-20261114/000756/`。不要复制到 Git 或 Release。

更新前 13 项公网真实推理全部完成（HTTP 200 + 正文 + 正常结束），不是只测 `/models`：

- 组 11：GPT-5.6 Sol、GPT-6.1 Sol、GPT-6.1 Terra（Responses）。
- 组 13：Claude Fable 5.2、Sonnet 5.5、Haiku 5.5（Messages），Opus 5.5（Responses）。
- 组 15/19：GLM-5.3 Flash（Anthropic / OpenAI 分别验证）。
- 组 28：Grok-4.6、Grok-4.7（现有用户 Key，Chat）。
- 组 17：Gemini 4 Hydrogen、Carbon（Chat，含已兼容的思考别名字段）。

这是分组/协议的代表性验收，不代表每一把上游 Key、每一个型号或每一种工具协议都被穷举验证。
本轮没有批量清冷却、恢复欠费 Key、改账号凭证或扩大模型能力白名单：

- GPT 2256/2273/2274 的 429 正文明确余额耗尽，已有摘流保留。
- Claude MAX 订阅组 5/6 仍没有健康库存；不把组 13 的按量 Key 塞入订阅组。
- Gemini 高负载 503 是上游容量信号，不应标为永久无效 Key。
- DeepSeek 组 20 当前没有账号；不创建虚假健康库存。

## 测试与发布门禁

各代理先用回归复现，再实现最小修复；主代理整合后验证：

- `go test -tags unit`：apicompat、openai、claude、service、handler、middleware。
- 计费取消/半开许可、GLM reset 专项 race 测试。
- `go vet` 和 `git diff --check`。
- 保留前端，仍从新干净 tag 构建同一提交的嵌入资源。
- 正式 GitHub Release + checksums，通过指定版本 `install.sh upgrade` 上线，不 scp 覆盖运行文件。
- 上线后重复分组探活，并检查工具结束事件、模型列表和生产配置/策略/迁移保护值。

## 暂缓

不迁入官方整套新调度架构、OAuth 伪装重构或图片后台任务；没有新的官方 main/model 变更就不猜测添加模型。
`content_block_start` 携带完整工具 input 的第三方流差异未纳入本次独立补丁。
