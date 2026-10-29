# Gemini 稳定性复盘：2026-10-29

## 生产基线

- `ssh ib` 实查：`0.1.337 / ac8e43a27c6286eb2089083fdf6c4d4c887c2737`，服务 active，NRestarts=0。
- 结论来自 journal、ops_error_logs、入站请求结构和实际推理，不以 `/models=200` 判断可用性。
- 原始请求、响应签名和账号备份仅保存在 VPS `/root/sub2api-gemini-20261029/`，不提交凭证、用户提示词、工具参数或签名。

## 1. 工具续轮 400：调用方丢失 Google 签名

真实链路：

- 18:23:35，请求 `f38af3f9-31b3-4d92-a1ae-21df471516bf`，1833，`gemini-3.8-flash`，第一轮 200。
- 18:23:37，请求 `e43f5c8c-5cdc-4615-8473-9b608cdc6783`，同账号/模型，续轮 400。
- 入站 assistant 工具调用 `skill / call_275406` 只有 `id/type/function`，缺少 `extra_content.google.thought_signature`，请求没有截断。
- 调用方 User-Agent 为 `node`；仅凭该值不能确定具体客户端。
- 18:27:45，又一真实续轮 `97cb6048-e69d-419e-bac2-4e4c1e9cce12` 出现相同错误，不能视为随机网络失败。

A/B 实测：

| 测试 | 结果 |
| --- | --- |
| 经网关流式获取工具调用 | 200，真实签名非空 |
| 原样保留签名，工具结果续轮 | 200 |
| 删除签名后经网关续轮 | 400，明确提示回传签名 |
| 同一缺签名请求直连 Google | 400，同样缺签名 |

这条原生 Chat 转发没有删除签名；它收到的历史本来就没有签名。换号、清空冷却、关闭思考或伪造签名都不是修复。需要调用端在组装 assistant.tool_calls 历史时原样保留供应商扩展，或使用支持 Google 原生工具历史的适配器。尚未确定具体调用端，不能宣称客户端已修好。

## 2. 工具调用结束状态不规范：网关补齐兼容

真实流式响应先返回含签名的 `delta.tool_calls`，最后一个无工具字段的 chunk 却返回 `finish_reason=stop`。

本轮修复：

- Gemini 非流式返回存在 message.tool_calls 时，将 stop 转为 tool_calls。
- 流式按请求内的 choice.index 记录是否收到过工具调用，再修正该 choice 的最终 stop。
- 不跨请求共享状态，不改变普通文本 choice。
- 不覆盖 length/content_filter；保留所有工具参数、签名和其他扩展。
- 不修改 OpenAI/Grok/其他平台。
- 补入站签名及未知扩展保留、跨 chunk/多 choice、失败结束语义回归。

## 3. 75 秒超时与账号池差异

另一次诊断请求 `aa3e9621-8f0f-4561-a6e2-7ecc11a7f92c`：

1. 1803 返回 402 后换号；
2. 1806 等待上游，客户端 75 秒探测超时取消；
3. 网关记录 context canceled / 最终 502。

该超时确实复现，但不能把客户端主动取消当作 Key 无效。

对组 17 中实际声明支持 `gemini-3.8-flash` 的 9 个 active/schedulable 账号逐个直连，最大并发 4、单次上限 18 秒：

| 账号 | 结果 | 说明 |
| --- | --- | --- |
| 1812、1833、1837、1842 | 200 且正常结束 | 本次约 1.3–2.0 秒 |
| 1803、1841 | 402 | `Your prepayment credits are depleted.` |
| 1806、1809 | 503 | Google 明确报告模型高负载 |
| 1820 | 无 HTTP 响应，18 秒超时 | 不等于凭证失效 |

随后对 1803、1841 分别使用 Google 原生 generateContent 验证 `gemini-3.8-flash` 和低价探活模型 `gemini-3.5-flash-lite`，四次均为同一 402 余额耗尽原文，排除仅 OpenAI 兼容端点故障或仅 3.8 Flash 权限问题。

仅本次抽样不能推导长期成功率，也不能把其他模型的权限等同于 3.8 Flash。

## 4. 余额耗尽不再按短暂故障反复入池

此前 Gemini 402 落入通用 TemporaryCooldown，默认冷却后自动再次尝试，无法解决真实预付费耗尽。

本轮仅对 Gemini API Key、HTTP 402 且错误正文精确以 `Your prepayment credits are depleted.` 开头（或等于无句点短句）的响应，复用既有 SetError + schedulable=false 路径。它代表需要人工充值/恢复，不表示密钥被撤销；充值后可以真实测试恢复。

- 未识别的 402 仍暂时冷却，不扩大到所有账单相关文案。
- 其他平台、参数 400、工具签名 400 不受影响。
- 503 high-demand 仍仅短暂冷却对应模型，不停用整个 Key。
- 不将超时账号永久标错，不清空全部账号状态，不变更模型映射、分组或策略。

## 5. 另一条 429 不是同一故障

2026-10-28 01:29:36 的 `gemini-3.1-pro-preview` 返回 Free Tier quota limit=0，涉及日/分钟请求和输入 token。该 ops 记录上游为 429、最终 HTTP 为 200，须与换号后的整体结果区分。免费项目没有该模型配额，不应靠重复重试认为额度会恢复。

## 验证与上线

- 结束原因回归在修改前已失败，复现 stop/tool_calls 差异。
- `go test -tags unit ./internal/service ./internal/handler ./internal/handler/admin -count=1` 全部通过；`go test -tags embed ./internal/web -count=1` 通过。
- 正式版本与部署验收在实际完成后补充。
- 生产配置、分组、迁移和凭证映射在升级前保全；不因发布删除历史备份。

## 剩余事项

1. **客户端签名丢失尚需修调用端**：当前仅可识别为 node，未定位应用名称/源码，不修改不明本机配置。
2. **上游高负载/超时不能由协议补丁消除**：未盲目缩短所有模型的响应头超时，以免截断高思考长任务；也未把慢账号永久禁用。
3. 不对失效/未启用服务的历史 Key 做批量恢复，不以数百个导入账号的总数充当可用模型库存。
