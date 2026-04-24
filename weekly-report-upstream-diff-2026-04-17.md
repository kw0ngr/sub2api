# 当前仓库与原版 `sub2api` 差异分析周报

## 1. 对比基线

- 当前仓库：`origin/main`，仓库地址为 `kw0ngr/sub2api`
- 原版仓库：`upstream/main`，仓库地址为 `Wei-Shaw/sub2api`
- 当前仓库头部提交：`f4936d9f`，版本号 `v0.1.116`
- 原版仓库头部提交：`00c08c57`
- 双方共同基线提交：`b384570d`
- 分叉状态：
  - 当前仓库相对原版 **额外新增 22 个提交**
  - 当前仓库相对原版 **落后 156 个提交**

> 结论：当前仓库已经不是“紧跟原版的小改动分支”，而是一个**带有明确定制方向的长期 fork**。  
> 它在关键网关链路上做了深度改造，但同时与原版主线功能演进出现了明显分叉。

## 2. 差异规模概览

- 当前 fork 独有差异共涉及 **76 个文件**
- 代码变更规模为 **+4651 / -446**
- 变更主要集中在后端网关与服务层：
  - `backend/internal/service/` 占全部差异文件的 **44.7%**
  - `backend/internal/handler/` 及 `backend/internal/handler/admin/` 合计约 **11.7%**
  - `deploy/` 占 **7.8%**
  - 前端管理端占比相对较小，主要用于支撑后台运维能力

### 改动热点文件

| 文件 | 变更量 | 说明 |
| --- | ---: | --- |
| `backend/internal/service/gateway_service.go` | +653 / -161 | 核心网关链路深度改造，属于本次 fork 的主战场 |
| `backend/internal/handler/admin/account_apikey.go` | +518 | 新增后台 API Key 批量导入与检测接口 |
| `backend/internal/service/apikey_health.go` | +432 | 新增 API Key 健康检查、禁用、冷却判定逻辑 |
| `backend/internal/service/ratelimit_service_apikey_test.go` | +391 | 为限流与健康判定补测试 |
| `backend/internal/service/upstream_fault_mapper.go` | +189 | 新增上游错误映射与脱敏处理 |
| `frontend/src/components/admin/account/RawKeyImportModal.vue` | +172 | 新增管理后台 API Key 批量导入界面 |
| `backend/internal/repository/startup_sql_fixes.go` | +151 | 启动时自动修复数据库兼容性问题 |

## 3. 当前仓库相对原版的核心定制方向

从提交记录和代码改动看，当前 fork 的改造重点并不是通用后台功能，而是围绕以下 4 个方向展开：

1. **面向 fork 的独立发布与安装体系**
2. **面向运营侧的 API Key 资产批量接入与健康管理**
3. **面向 OpenAI/Claude 链路的错误处理、检测规避与兼容增强**
4. **面向 Anthropic OAuth 场景的 `auth2api` / Claude Code 拟真接入**

下面按模块拆解。

## 4. 本仓库相对原版的新增能力

### 4.1 Fork 独立化：安装、发布、文档改为当前仓库自维护

当前仓库已经将部署和发布链路从原版仓库切换到 fork 自己维护，主要体现为：

- `README.md`、`README_CN.md`、`README_JA.md` 中的安装与克隆地址，统一从 `Wei-Shaw/sub2api` 改为 `kw0ngr/sub2api`
- `deploy/install.sh` 的 `GITHUB_REPO` 改为当前 fork，并增加“fork 还没有发布 release 时给出明确提示”的逻辑
- `.github/workflows/release.yml` 与 `.goreleaser.simple.yaml` 调整为：
  - 手工触发时允许 simple release
  - 正式 tag 发布必须产出完整安装资产
  - 避免因为“仅发镜像、不发二进制”导致安装脚本不可用

**业务意义：**

- 当前仓库已经具备独立发布能力，不再依赖原版 release 资产
- 部署链路更可控，但也意味着后续版本发布、资产完整性、安装脚本可用性都需要 fork 自己兜底

### 4.2 新增 API Key 批量导入能力，强化后台运营效率

当前 fork 新增了完整的 API Key 批量导入与校验链路，原版中没有这一套定制实现。

主要能力包括：

- 后台新增原始 Key 文本批量导入接口
- 支持根据 key 前缀自动识别平台：
  - `sk-ant-` 识别为 Anthropic
  - `AIza` 识别为 Gemini
  - `sk-` 识别为 OpenAI
- 支持导入后立即校验有效性
- 支持跳过默认分组绑定
- 支持按 `platform + api_key + base_url` 做去重索引，避免重复导入
- 前端管理端新增批量导入弹窗，能够展示逐行导入结果、有效/失效/重复状态

涉及文件：

- `backend/internal/handler/admin/account_apikey.go`
- `backend/internal/service/apikey_health.go`
- `frontend/src/components/admin/account/RawKeyImportModal.vue`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/api/admin/accounts.ts`

**业务意义：**

- 明显降低 API Key 资产录入与维护成本
- 更适合“从外部批量拿 Key 后快速入库和筛选”的运营模式
- 相比原版，当前 fork 在“Key 资产运维”这个场景上更偏实战

### 4.3 新增 API Key 健康检查、自动禁用与冷却策略

这是当前 fork 相对原版最明确的一块增强能力。

当前实现新增了统一的 API Key 健康判断框架，可根据不同平台的上游错误码与错误文本，判断账号应该：

- 忽略
- 标记为有效
- 永久禁用
- 临时冷却

其判定逻辑已经对不同平台分别细化：

- **OpenAI**
  - 401 / 402 大多视为永久失效
  - 429 会区分“额度耗尽”和“临时限流”
  - 400 / 403 会结合错误码与错误消息识别是否是账号停用、项目停用、billing hard limit 等
- **Anthropic**
  - 401 直接永久失效
  - 403 会区分“账号权限/认证问题”和“模型访问限制”
  - 400 中增加对 credits 不足等情况的判断
- **Gemini**
  - 403 会结合 Google API 错误 reason 判断是否为 billing disabled / project disabled / consumer suspended
  - 400 会识别 `API_KEY_INVALID`、`FAILED_PRECONDITION`、未启用服务、未开 billing 等情况

同时，当前仓库还补了配套治理能力：

- 增加 API Key 唯一约束迁移：`082_add_apikey_unique_constraint_notx.sql`
- 调整 `ratelimit_service.go`，将限流、冷却、禁用决策与真实连接检测更紧密地对齐
- 增补了较多测试，说明该能力已不只是概念验证，而是进入了可维护阶段

**业务意义：**

- 当前 fork 更适合高频、批量、动态质量波动的 Key 池场景
- 可以减少无效 Key 持续重试带来的资源浪费
- 可以降低因为错误判定过于粗暴而导致“误封可用 Key”的概率

### 4.4 OpenAI 检测规避与错误透传策略被重构

当前 fork 在 OpenAI 相关错误处理链路上做了明显强化，核心目标是：

- 避免直接暴露上游原始错误体
- 对上游异常进行统一映射和脱敏
- 减少因错误透传导致的检测暴露面

这部分改动主要包括：

- 新增 `upstream_fault_mapper.go`
  - 将上游错误统一映射为内部错误类型
  - 对 OpenAI Responses、Chat Completions、Anthropic Messages 分别输出适配后的错误格式
- `error_passthrough` 能力调整
  - 明确废弃 `passthrough_body`
  - 新增迁移 `083_disable_error_passthrough_body.sql`
  - 后台接口中将该字段视为 deprecated，运行期强制关闭
- 新增 `084_migrate_openai_passthrough_key.sql`
  - 将历史字段 `openai_passthrough` / `openai_oauth_passthrough` 迁移到统一的新字段 `forward_passthrough_only`
- 启动时自动执行 `startup_sql_fixes.go`
  - 用于补做数据库兼容修复，降低升级失败风险

**业务意义：**

- 当前 fork 更强调“统一对外错误口径”和“上游错误脱敏”
- 对于敏感上游链路，这能降低原始响应直接泄露带来的兼容和识别风险
- 但也意味着与原版在错误处理语义上已经不完全兼容

### 4.5 深度集成 `auth2api` / Claude Code 拟真逻辑

这是当前 fork 最有辨识度的一项改造，也是与原版分叉最深的技术点。

从 `gateway_service.go`、`identity_service.go`、`header_util.go` 等实现来看，当前 fork 已经围绕 Anthropic OAuth 请求做了较完整的 Claude Code 拟真处理，包括：

- 维护 OAuth 账号的请求指纹缓存
  - `User-Agent`
  - `X-Stainless-*` 系列头
  - 客户端版本升级时按 merge 语义更新，而不是简单覆盖
- 保持请求头大小写与顺序更接近真实 Claude CLI 抓包特征
- 为 OAuth 请求补齐或重写：
  - `X-Claude-Code-Session-Id`
  - `metadata.user_id`
  - `anthropic-beta`
  - system 中的 billing header / Claude Code 前缀块
- 动态计算 `anthropic-beta` 头
  - 根据模型类型、是否结构化输出选择不同 beta token 组合
- 引入 system cloaking
  - 将固定 OpenCode 身份文本重写为 Claude Code 风格
  - 在 system 中插入 `x-anthropic-billing-header`
- 对 session 与 metadata 做稳定化处理
  - 同一 API Key 下维持稳定 session
  - 通过 account、device、session 等信息重写 `metadata.user_id`

**业务意义：**

- 当前 fork 已经不是单纯的网关转发，而是在做面向 Claude OAuth 链路的请求拟真与协议补齐
- 这类能力通常服务于兼容性、风控规避或特定客户端适配场景
- 这也是后续与原版合并最容易发生冲突的区域

## 5. 当前仓库相对原版缺失的上游能力

虽然当前 fork 有 22 个定制提交，但它同时落后原版 156 个提交，意味着原版近阶段的大量功能和修复还没有进入当前仓库。

结合上游提交记录，当前仓库至少缺失以下几类上游能力：

### 5.1 渠道管理、模型映射、计费与使用记录增强未同步

上游近阶段投入较多的方向，是“渠道管理系统”的持续增强，包括：

- 渠道管理全链路
- 模型映射
- 多模式定价
- 模型限制
- 使用记录计费模式展示
- image output token billing
- group/messages 调度映射
- instructions 模板注入

这说明：

- 原版在“统一计费、渠道编排、管理后台能力”上推进较快
- 当前 fork 没有跟进这一大块，整体更偏“接入与兼容改造”，而不是“平台化运营增强”

### 5.2 登录与账号体系新能力未同步

上游已经合入：

- OIDC 登录支持
- 分组消息调度配置
- 更完善的 admin/service 相关管理能力

当前仓库尚未吸收这些能力，说明账号体系和企业接入能力相对原版偏弱。

### 5.3 支付、通知、WebSearch 等新模块未同步

从上游提交可以看到，原版已经继续演进出以下方向：

- 完整支付系统与多支付提供方支持
- 充值手续费、退款控制、金额展示优化
- 余额低提醒、额度通知系统
- WebSearch 配额增强与 Anthropic API Key 搜索仿真
- Admin Dashboard / Usage 页面成本展示增强

这部分在当前 fork 中尚未体现，说明当前仓库在“运营平台能力”上落后于原版。

### 5.4 上游近期稳定性修复和安全更新未完全同步

当前仓库还未合入原版近阶段的一批稳定性与安全修复，例如：

- Go 版本升级至 `1.26.2` 以修复标准库 CVE
- Redis 调度快照与缓存一致性修复
- OpenAI WS 相关状态保留与误判修复
- 非流式空输出修复
- refresh token race condition 修复
- base64 空图片处理、Gemini 搜索 grounding 修复
- KYC / 限流 / watermark / outbox 等多个边界问题修复

这意味着：

- 当前 fork 的定制能力更强
- 但“通用稳定性”和“主线持续修复收益”并没有及时继承

## 6. 风险判断

### 6.1 与上游分叉已经进入“高合并成本”阶段

风险最大的文件是 `backend/internal/service/gateway_service.go`。  
当前 fork 在这里做了重度定制，而上游近期也持续在同一层做功能演进与修复。

这意味着后续如果要直接 rebase 或整体 merge：

- 冲突会集中爆发在网关主链路
- 逻辑冲突不只是代码冲突，还包括错误处理语义、header 处理、beta 策略、请求规范化方式的差异

### 6.2 当前版本号不能直接代表“比原版更新”

虽然当前仓库版本号已经到 `0.1.116`，而上游提交记录里能看到 `0.1.110` 等版本字样，但两边版本线已经分叉维护，**不能仅凭版本号判断谁更新**。

更准确的判断依据是：

- 提交分叉状态：当前 fork 落后上游 156 提交
- 功能线方向：当前 fork 偏定制接入，上游偏平台能力扩展

### 6.3 数据库与部署链路已经出现 fork 专属维护成本

当前仓库新增了：

- fork 专属 release 逻辑
- fork 专属 install 脚本
- fork 专属启动时 SQL 修复
- fork 专属数据迁移键名兼容逻辑

这类改造能提升独立可运维性，但也会带来额外负担：

- 发布资产必须自己保证完整
- 数据库升级路径需要自己维护兼容
- 后续若回归原版主线，迁移策略需要重新核对

### 6.4 提交规范性一般，后续追溯成本偏高

当前 fork 中存在部分提交信息较弱，例如：

- `1`
- `2`
- `Fix Bug`
- `git nore`

这不会直接影响运行，但会明显增加后续的：

- 需求回溯成本
- 问题定位成本
- 合并决策成本

## 7. 结论总结

综合来看，当前仓库与原版的区别可以概括为一句话：

> **当前仓库是在原版 `sub2api` 基础上，围绕 API Key 运维、OpenAI/Claude 检测规避、Anthropic OAuth 拟真接入做了深度定制的长期 fork；但与此同时，它已经明显落后于原版近期的平台化能力建设与稳定性修复。**

如果从周报视角提炼，本周可重点强调以下几点：

- 已确认当前仓库并非简单同步版，而是具有明确业务目标的深度 fork
- fork 侧已形成 4 条核心改造线：
  - 独立发布与安装体系
  - API Key 批量导入与健康治理
  - OpenAI 错误处理与检测规避增强
  - `auth2api` / Claude Code 拟真接入
- 与原版差异已扩大到“22 个定制提交 vs 落后 156 个上游提交”
- 后续维护重点不再是“是否继续小修小补”，而是需要明确选择：
  - 继续作为定制 fork 独立演进
  - 还是分阶段吸收上游能力，控制分叉风险

## 8. 建议的下周动作

### 方案建议

- **短期建议：不要直接整体 rebase 上游**
  - 当前网关链路改动过深，直接整体合并风险较高
- **更合理的策略是“按主题选择性回收上游能力”**
  - 优先吸收安全与稳定性修复
  - 再评估是否需要合入渠道管理、OIDC、支付、通知等平台能力

### 建议优先级

1. 优先补齐上游安全和稳定性修复
2. 梳理 `gateway_service.go` 中 fork 专属逻辑，尽量模块化、隔离化
3. 为 fork 专属改造补充一份正式设计说明，避免知识只停留在代码里
4. 后续提交规范统一，减少 `Fix Bug`、`1`、`2` 之类不可追踪提交

---

本文档适合作为“当前仓库与原版差异梳理”周报材料直接引用，也可拆分为“本周已完成事项 + 风险与下周计划”两部分使用。
