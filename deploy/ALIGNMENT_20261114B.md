# 0.1.346：Gemini 故障复盘与剩余官方协议修复

## 发布边界

- 2026-11-14 再次 fetch 官方 main，仍为 `530c5e3a4`（0.2.21）。
- 本分支起点/生产基线 `010aa60ff`（0.1.345）。
- 多代理各自拥有独立代码范围，主代理检查生产日志、整合、回归与发布。
- 保留工作区的其他并行任务改动；本次发布只包含明确列出的补丁，不打包未提交前端/配置清理。
- 保留 Grok 策略/CLI、Gemini 签名缓存与思考别名、GLM 两协议、GPT Sol max、分组定义及已执行迁移。

## Gemini 本次真实故障

2026-11-14 20:02:40（UTC+08），Carbon 唯一已验证账号 1842 的一次高思考请求在等待响应头 90 秒后超时。
原逻辑设置了该模型 5 分钟退避，到 20:07:40 恢复。
此后 16 条 `503 Service temporarily unavailable` 没有选到账号、没有上游状态码，
不是 16 次 Google 503，也不是 16 把 Key 失效。

日志中另外两条 499 是客户端取消；三个请求体读取失败未携带完整请求，不能凭空归因于上游 Key。
Hydrogen 有一条上游 high-demand 503 后换号成功并返回 200，不应把已恢复请求当作整次失败。

修复：

- 本地响应头超时后的 **模型级退避由 5 分钟缩短为 30 秒**。
- 保留 60/90 秒普通等待预算和 90/120 秒高思考等待预算，不增加无限重试。
- 真实 429 或已有更晚模型配额重置继续优先，客户端取消不冷却账号。
- Chat / Responses 请求在读入站 body、转发失败、读上游 JSON body 时准确识别入站 context 取消：
  未发响应记 499、不追加 502、不惩罚调度；已开始 SSE 不追加伪造终止错误。
- 活跃客户端的真实上游读取错误/内部超时不被隐藏，仍保留 502/504。

容量核查：对组 17 当前 31 个 active/schedulable Key 真实探活，
其他账号主要返回 Carbon 免费层 429；1842 在其中一次 24 秒探活超时。
随后对 1842 使用足够等待时间分别验证：

- Carbon OpenAI low：200（5.34s）。
- Carbon 原生 generateContent：200（9.81s）。
- Carbon OpenAI high / `max_completion_tokens=128000`：200（8.96s）。

没有因为超时删除或禁用 1842，也没有给未通过推理的免费 Key 添加 Carbon 能力。
该型号仍只有一个已验证生产账号，短退避减少故障放大，但不等于增加了上游额度或消除了上游容量波动。

## 其他已整合修复

| 范围 | 依据 | 修复 |
| --- | --- | --- |
| Claude / GLM / DeepSeek Responses 工具流 | 官方 `97bdde313` | 保存 `content_block_start.input`；真实 delta 覆盖 seed，不能拼出两个 JSON；无 delta 时补发真实完整参数，delta/done/item/completed 一致，空工具回退 `{}`。 |
| GPT HTTP Responses | 官方 `a4ea8059b` | 400 `thinking_signature_invalid` 且错误明确说明 encrypted content 校验/解密失败时，复用既有加密推理清理流程，最多重试一次；保留 summary、工具配对与 max。普通 400 不重试。 |
| Grok API Key 健康分类 | 本地遗漏，与原运行时 helper 对齐 | 明确无效/错误 API Key 与 401 在健康探测、手动测试和运行时统一停调；内容拒绝、模型无权限、Build 探活噪音不因此禁号。 |

## 证据与验证

VPS 含凭证/请求的证据只保留 root-only：
`/root/sub2api-align-20261114b/194902/`，不进入 Git 或 Release。

- 后端 apicompat/openai/claude/service/handler/middleware 单元测试。
- 新工具流、HTTP 签名恢复、取消请求和短退避专项 race。
- 修复前后代表性模型池真实推理，额外重放 Gemini ZCode 流式请求。
- `go vet`、`git diff --check`、安装脚本离线测试。
- 从干净 tag 构建 Linux amd64 + 同一提交嵌入前端；正式 Release/checksum/install upgrade 发布。
- 验证配置、分组、Grok policy、迁移保护值与运行文件 hash。

本轮不批量恢复失效/欠费账号，不创建用户 Key、不清理其他任务文件，不引入新的大架构或猜测新模型。
