# 0.1.342：官方独立修复与模型池纠偏

## 基线和范围

- 2026-11-13 重新 fetch 官方 `Wei-Shaw/sub2api`；main 为 `530c5e3a4`（0.2.21）。
- 本地发布基线为 `0bf82cd5b`（0.1.340）。不整体 merge，不替换本分支协议适配、Grok 策略或数据库迁移。
- 保留 `.omo/`、生产配置、既有备份、分组定义、Grok 策略和 Gemini 长会话签名缓存。

## 引入的高价值修复

| 官方参考 | 本分支落点 |
| --- | --- |
| `4ac713c4d` | 调度账号 hydrate 失败，立即释放已占用的并发槽，避免等到 TTL 才恢复。 |
| `36484727c` | Gemini → Messages 非流式转换不把 `thought=true` 的文字混入正文；工具与计费用量保留。 |
| `f2b0a32b1` | Responses SSE 补全输出保留工具 `namespace` 和 `encrypted_function_args`，包括显式 `null` / `[]`。 |
| `85f43b640` | Responses 工具循环追加 developer/system 提醒，不再改变会话种子；保留现有 Chat/Gemini 种子格式。 |
| `63f117229` | 非法 UTF-8 请求体在 JSON 改写与调度前返回 400。 |
| `f10a45c03` | 非法 UTF-8 Key 不进数据库；鉴权查库期间客户端断开记 499，不伪报 500。 |
| `eb2da3448` / `3e8c2ee3f` 及前置 Claude 5.5 实现 | 有选择地适配新模型目录、effort、自适应思考、签名回传、定价兜底，而非搬入新版架构。 |

附带修复：

- SSE 转 JSON 后显式重置 Content-Type，避免继承上游 `text/event-stream`。
- Claude 模型别名在协议转换前解析，避免按旧模型选择手动思考；转换参数不支持时返回 400，不访问上游。
- Claude 原生签名历史、Responses 非流式及流式签名历史均保留；旧 Claude 与 DeepSeek/GLM 的思考回传行为不变。
- 0.1.341 未部署；发布前补齐非流式桥接的 `signature_delta` 累积与别名识别后，以新 tag 0.1.342 上线，不覆写旧 tag/资产。
- DeepSeek 兼容渠道的 `403 + code=INSUFFICIENT_BALANCE` 明确归类为余额故障，不当普通临时限流。

## 模型与计费

新增目录/原名映射预设：Fable 5.2、Mythos 5.2、Opus 5.5、Sonnet 5.5、Haiku 5.5。

目录支持不等于生产账号有权限。Mythos 5.2 上游尚返回 404，本次不为生产账号添加其能力。
GPT 既有 GPT-6.1 Terra/Sol 与 GPT-5.6 Sol 保留，不把 `max` 改成 `xhigh`，不以旧型号冒充新型号。

新 Claude 固定型号仅在缺少精确目录定价时使用本地官方兜底，渠道自定义价格仍优先。
包含缓存读取、5m/1h 写入与 Haiku 5.5 的 100K 长上下文阶梯；不把新 Haiku 错算成 Claude 3 Haiku。

## 生产配置操作门禁

2026-11-13 从 VPS 真实直连推理重新取证：

- Claude：22 个未分组 Key 通过；仅把各自实际成功的型号白名单补入 **组 13（Claude 三方Key）**，不加入 MAX 订阅组 5/6。
- GPT：1463、1469、1490、1518、1881 明确对 `gpt-5.6-sol` 无权限；仅删该映射，不停用 Key 或删除其它能力。
- Gemini：1812 项目暂停、1833 预付余额耗尽、1837 无效 Key；只摘除这些确认的硬故障。503 high demand 不禁用整账号。
- DeepSeek 3210：上游明确余额不足。修复分类与摘流不等于补充额度，不能宣称已恢复推理。

操作使用管理 API；执行前保全账号与组成员，写入前断言 credentials 未被并发修改。
证据和含凭证备份仅保留 VPS root-only 目录 `/root/sub2api-align-20261113/`，不进入 Git 或 Release。

## 回归与发布

- 后端 `go test -tags unit` 覆盖 apicompat、claude、service、handler、middleware 五包。
- 前端 `vue-tsc` 与 useModelWhitelist 定向测试；构建嵌入前端。
- 新 tag 的干净源码构建 Linux amd64 正式资产，注入 commit/date/release 标记。
- 按 `RELEASE_SERVER_UPDATE_RUNBOOK.md` 上传正式 Release，checksum 校验后运行指定版本 `install.sh upgrade`。
- 发布后检查真实模型推理、流式工具调用、运行文件 hash、配置/分组定义/策略/迁移保护值。

## 本轮暂缓

新版官方整体调度/请求体读取架构、OAuth CLI 伪装大改及 GPT Image 2.6 全套链路未整体迁入；
不为对齐引入无关迁移，也不恢复上游已失效或无余额的凭证。
