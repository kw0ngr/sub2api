# 官方对齐与日志复盘（2026-10-27）

## 基线与范围

- 官方远端：`Wei-Shaw/sub2api`，已 `git fetch upstream --prune`。
- 本次核对 `upstream/main`：`15701ba187af5827d740901d53421bd69ed97717`，2026-10-27 19:00 +08，版本线 `0.2.18`。
- 原生产：`0.1.335` / `89092f4fcb96e377b7224476ee812d7444463288`；二进制 SHA-256 `f87300658d8ac8169c351a6646210f3012bc90ef35ceb9b3377ab0cd5e99e7a8`。
- 不整库 merge，不修改已执行迁移，不删除账号/生产定制、策略、配置或历史备份。新版本须从包含现有定制的完整提交构建。

## 本轮采用

### 1. GPT-6.1 Terra

参考官方仓库提交 `34236674223877d265bb5dbc071277d04ded0d34`，按本分支的现有兼容层实现，而不是覆盖官方新版文件：

- 补齐 API 默认目录、管理端映射预设、OpenCode 配置和本地 Codex manifest。
- 支持 `none / low / medium / high / xhigh / max`，默认 medium。保留 GPT Sol 原有 max 与 none 校验差异。
- 工具调用要求 Responses；Chat-only 渠道在发送上游前明确拒绝工具请求，不伪装成功。
- 标准输入/缓存读取/缓存写入/输出：$1 / $0.05 / $1.25 / $5 每百万 token；Fast 2 倍；超过 272K 输入时按整次请求应用输入/缓存 2 倍、输出 1.5 倍。
- 上游 `/models/gpt-6.1-terra` 返回 `created=1791908780`；目录时间使用该值，不把合并时间冒充发布时间。
- VPS 上 8 个候选 Key 的 `/v1/responses` 均 `200 + completed + OK`：425、1497、1874、2242、2245、2250、2258、2350。2245 另验证 none/max 均成功。只有通过推理验收的 Key 可以增加生产映射。

来源：[模型及协议](https://developers.openai.com/api/docs/models/gpt-6.1-terra)、[定价](https://developers.openai.com/api/docs/pricing)。

### 2. Gemini / Antigravity 用量非负

参考官方提交 `9c41b32695c4aa5d2673f228097762cba14e64f0`：

- 原生兼容用量提取、Antigravity 非流式和流式的 4 处输入换算加下限 0。
- 上游只报缓存命中、未报 promptTokenCount 时，保留缓存 token，不产生负输入 token。
- 数据库近 30 天未发现负输入/负费用；这是预防性补齐，不宣称已经发生财务损失。

### 3. 日志驱动：GLM 1310 重置时间

- 近 7 天错误记录：42 条 GLM 429。1784 多次明确返回 `2026-10-28 05:51:54` 才重置，但本地 `rate_limit_reset_at` 只延后 15 分钟。
- 新逻辑仅对 GLM 1310 解析正文中的中国标准时间，避免把每周/月额度耗尽当短暂并发限流。
- 合法未来重置时间优先于默认冷却；明确 Retry-After 更晚时遵循更晚值。非法/过期时间回退旧逻辑。
- 不为账号充值，不提前恢复配额，不联动相同 Key 的其他协议账号。

### 4. 生产模型能力纠偏

- 近 7 天有 35 条 OpenAI 403 记录，主要是 gpt-5.5 项目无权限；部分最终 200 是换到其他账号成功，不代表被拒 Key 可用。
- 1463、1469、1518、1881 均重新直连验证 `403 model_not_found`。
- 管理 API 操作前逐账号备份，仅移除明确的 `gpt-5.5 → gpt-5.5` 映射，不禁用账号、不改其他模型或分组，不自动映射到新型号。
- GLM 6 条 400 为 1301 内容拒绝，不当作账号失效；Grok 3 条无模型 400 为读请求体失败，现有证据不足以认定上游模型故障。

## 其他模型核对

- Gemini 官方截至 10-26 的新条目是付费新项目 Generate Content 默认存储行为变化，最新文本旗舰仍是已支持的 `gemini-4-argon`；本轮未全局改变存储设置。
- 实际 xAI 目录仍列 grok-4.7，同时出现 imagine-image-2.0 / video-1.5 系列。未实测生成完成、定价和媒体协议前，不把“列在 models”当生产生成能力，也不把这些 ID 盲加到文本池。
- 第三方 DeepSeek 3210 的 `/models` 当前 403，不能据此恢复或猜测新模型；不更改现有 V4.1 Flash 映射。

来源：[Gemini 更新记录](https://ai.google.dev/gemini-api/docs/changelog)、[xAI 模型](https://docs.x.ai/developers/models)、[DeepSeek 更新](https://api-docs.deepseek.com/updates)。

## 高价值后续对齐（本轮未采用）

| 优先级 | 官方变更 | 下一步与暂缓原因 |
| --- | --- | --- |
| 高 | DOMPurify 安全更新 `828626804` | 定向更新依赖及 lock，验证所有 HTML/Markdown 展示；不为新模型兼容顺带替换整套前端依赖。 |
| 高 | Cloudflare 1010 请求级隔离 `e0fa4b94f` | API Key 和 OAuth 错误处理入口不同，需覆盖自定义规则及所有健康写入分支，不能只复制 handle403。近期日志未观测到此类错误。 |
| 高 | Codex overload/身份收敛 `c591412c4` | 本分支保留了自己的 SSE 重试和 CLI 身份逻辑，须逐项比对，不覆盖式合并。 |
| 高 | Gemini 客户端断开后计费用量 `2b1f12938` | 涉及流式生命周期与回传计费，需按本分支实际转发路径加取消/部分用量测试后接入。 |
| 中 | Grok 模型目录审计 PR #8339 | 涉及快照/CAS、管理草稿、目录模式及响应模型审计；应独立模块对齐，避免影响已有 CLI/免费号和生产定制路径。 |
| 中 | 管理员 Key 操作/资源分权 `e445cc3e2` | 有数据库迁移和鉴权范围变化，应独立发布并回归现有管理员集成。 |
| 中 | 每图计费 `fd998129e` / `3ce09c8af` | 先明确多图端点与实际返回图片数，避免改变现有渠道价格。 |
| 中 | 调度快照写时复制 `38b72d734` | 本分支尚无同名 setAccountModelRateLimitSnapshot，不能直接 cherry-pick；先按共享 map 的真实写入点审查。 |
| 高 | 本地安装脚本校验下载失败仍继续 | 本次 checksums.txt 首次下载失败后脚本继续安装，已独立重下归档、校验归档 SHA-256 并确认包内文件与进程一致。后续应把“下载并严格校验”移至停服之前，失败时保留旧服务；不能仅把 warning 改成 exit 而让生产停在停止状态。 |
| 中 | 限额账号全部冷却后的客户端错误 | GLM 上游首次 429 后已正确冷却到真实重置；随后无候选账号仍返回 503，而非带 Retry-After 的 429。这是剩余体验问题，不是额度已恢复；后续统一无可用账号诊断时单独处理。 |

## 验证与证据

回归覆盖：Terra none/max/别名、Codex manifest、Chat-only 工具边界、Messages 映射、价格和长上下文；GLM 重置时区/非法时间；Gemini 原生与 Antigravity 流式/非流式缓存-only 用量。

- 本地临时证据：`/tmp/sub2api-align-20261027/`。
- VPS 原始探活与账号备份：`/root/sub2api-align-20261027/`，仅 root 可读。
- 上述备份含凭证，禁止提交仓库、输出全文或复制进报告。
- 发布和公网验收结果须在实际完成后补充，不以编译或 `/models=200` 代替。

### 已发布与线上验收

- 版本 `v0.1.336`，代码提交 `f564b84c6f057bda12bd482bc2c7c270126e1465`。
- GitHub Actions 仍被禁用，按用户此前批准的手动 Release 流程从干净 tag 构建，经 GitHub Release + `install.sh upgrade -v v0.1.336` 安装；仅发布 Linux amd64，未发布其他平台/Docker 镜像。
- 生产文件与运行进程 SHA-256 均为 `a25738d4f8116bea97f2f1c88851d527c09ad0510fd5bdda1dadba1349f306e4`。
- 安装时首次校验文件下载失败，已独立下载正式 archive 与 checksums.txt 完成 `sha256sum -c`，并确认 archive 内二进制与运行进程一致，未遗漏最终校验。
- 原生产备份：`/opt/sub2api/backups/sub2api-pre-0.1.336-20261027223952`，原有备份保留。
- 8 个通过上游推理验收的 Key 已增加 Terra 映射；公网 none Responses、max 流式 Responses、max Chat、函数工具调用均 `200` 且正常完成。
- Codex manifest 已显示 Terra 及 none/max；Gemini 3.8 Flash 回归 `200 / OK`。记录的成功推理约 1–3 秒，不代表长期延迟承诺。
- GLM 首次实测返回上游 429，并将 1784 的 `rate_limit_reset_at` 精确持久化为 `2026-10-28 05:51:54+08`；随后请求按现有无账号分支返回 503，无需再次撞上游。**这是配额保护验收，不是 GLM 推理成功验收。**
- 现有分组开关、策略哈希及全部数据库迁移校验与升级前一致；管理员策略/分组读取正常，设置页 HTML 与入口 JS/CSS 类型正常；本轮未改写策略或分组配置。
- 服务 `active`、`NRestarts=0`，自报版本/commit/build_type 与发布对应。
