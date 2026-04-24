# 上游能力按主题选择性回收方案

## 1. 目标

本文档用于指导当前 fork `kw0ngr/sub2api` 从上游 `Wei-Shaw/sub2api` 中**按主题、分批次、可控地回收能力**，避免直接整体 `rebase/merge` 带来的大规模冲突。

当前判断原则不是“缺什么补什么”，而是优先满足以下目标：

1. 优先回收**安全修复**和**通用稳定性修复**
2. 优先回收**与当前 fork 业务方向一致**的能力
3. 尽量避免在第一阶段碰撞 fork 的重度定制区
4. 所有回收动作都应可回滚、可验证、可拆批

## 2. 当前冲突面判断

从 `origin/main..upstream/main` 的差异看，最危险的冲突区域如下：

| 热点文件 | 上游未同步提交数 | 风险说明 |
| --- | ---: | --- |
| `backend/internal/service/gateway_service.go` | 43 | 当前 fork 与上游都在持续改，属于最高风险文件 |
| `backend/internal/service/openai_gateway_service.go` | 36 | OpenAI 链路分叉较深，直接合并容易出现语义冲突 |
| `backend/internal/handler/admin/setting_handler.go` | 26 | 几乎所有“配置类功能”都会撞到这里 |
| `backend/internal/service/account_test_service.go` | 3 | 与 API Key 健康检测、限流回流相关，需谨慎但可控 |
| `backend/internal/service/ratelimit_service.go` | 2 | 文件本身冲突不多，但它会影响账号调度行为 |

结论：

- **可以优先直接 cherry-pick 的主题**：安全补丁、部分独立稳定性修复、少量 OpenAI/运维补丁
- **需要分支验证后再回收的主题**：OpenAI 兼容增强、messages 调度映射、表格后端排序分页
- **当前阶段建议暂缓的主题**：渠道管理大改、支付系统、通知系统、WebSearch 大包、OIDC 登录

## 3. 推荐回收顺序

建议按 4 个波次执行，而不是一次性回收。

### Wave 1：安全与低耦合稳定性修复

这是最应该先做的一批，收益高、冲突低。

### Wave 2：OpenAI 链路稳定性与兼容性修复

与当前 fork 的业务方向最一致，但会碰到 `openai_gateway_service.go`，需要逐个验证。

### Wave 3：管理后台效率增强

如果你们近期更关心后台运维体验，可以单独回收。

### Wave 4：大功能主题按业务需要单独立项

这类主题不是“顺手 cherry-pick”能完成的，需要单独开分支、做迁移和联调。

## 4. 推荐立即回收的主题

## 4.1 主题 A：安全补丁

### 推荐理由

- 风险最低
- 与 fork 自定义逻辑耦合最小
- 不回收没有任何收益，回收则立即提升基线安全性

### 推荐提交

- `7060596a` `fix: bump Go from 1.26.1 to 1.26.2 to resolve 6 stdlib CVEs`
- `217b7ea6` `fix(deps): upgrade axios to 1.15.0 to fix GHSA-fvcv-3m26-pcqx`

### 影响范围

- Go 运行时与构建镜像
- 前端依赖 `axios`

### 风险

- 低

### 推荐执行方式

优先直接 `cherry-pick -x`：

```bash
git cherry-pick -x 7060596a
git cherry-pick -x 217b7ea6
```

### 回收后验证

- 后端 `go test ./...`
- 前端安装依赖并跑构建
- 检查 CI 配置是否仍与当前 fork 的 release 逻辑兼容

## 4.2 主题 B：运维与调度稳态修复

### 推荐理由

这些改动大多不直接碰 fork 的深度定制链路，但能提升线上可观测性和账号调度正确性。

### 推荐提交

- `6401dd7c` `fix(ops): increase error log request body limit from 10KB to 256KB`
- `3944b3d2` `fix: preserve openai ws flags in scheduler cache`
- `5d586a9f` `fix: 上游返回 KYC 身份验证要求时停止账号调度`

### 风险拆分

- `6401dd7c`：低风险，基本可直接拿
- `3944b3d2`：低到中风险，主要影响调度缓存
- `5d586a9f`：中风险，会改变账号停用/调度行为，需要确认与当前 API Key 健康判定策略是否一致

### 推荐执行方式

先单独 cherry-pick 前两个，第三个放在验证分支：

```bash
git cherry-pick -x 6401dd7c
git cherry-pick -x 3944b3d2
git cherry-pick -x 5d586a9f
```

### 回收后验证

- 检查调度缓存快照是否保留 OpenAI WS 标志位
- 检查上游返回 KYC 类错误时，账号是否进入预期不可调度状态
- 复核是否与当前 `apikey_health.go` / `ratelimit_service.go` 的禁用逻辑重复或冲突

## 5. 推荐第二批回收的主题

## 5.1 主题 C：OpenAI 链路稳定性修复

### 推荐理由

这批改动与当前 fork 的主业务方向最一致，收益高，但会碰撞 `openai_gateway_service.go`，因此不能盲目整批拿。

### 候选提交

- `6b646b61` `fix(openai): fail over passthrough 429 and 529`
- `3a07e92b` `fix(openai): do not normalize /completion API token based accounts`
- `7eecc49c` `fix(openai): do not normalize API token based accounts`
- `c5aac125` `fix(gateway): add content-based session hash fallback for non-Codex clients`
- `4fb16030` `test(gateway): add tests for content-based session hash fallback`
- `7451b6f9` `修复 OpenAI 账号限流回流误判：7d 窗口可用时不因 5h 窗口为 0 回写 429`

### 风险评估

| 提交 | 风险 | 原因 |
| --- | --- | --- |
| `6b646b61` | 中 | 只改 `openai_gateway_service.go`，与 fork OpenAI 透传链路相关，但价值高 |
| `3a07e92b` | 中 | 修改 chat completions/messages/ws 行为，需验证 API token 账号不被错误标准化 |
| `7eecc49c` | 中到高 | 触及 `openai_codex_transform.go` 和 `openai_gateway_service.go` |
| `c5aac125` | 中 | 改 session 计算逻辑，需确认不冲掉 fork 现有 session 隔离逻辑 |
| `7451b6f9` | 高 | 同时触及 `account_test_service.go`、`admin_service.go`、`openai_gateway_service.go` |

### 推荐执行顺序

建议顺序如下：

1. `6b646b61`
2. `3a07e92b`
3. `c5aac125`
4. `4fb16030`
5. `7eecc49c`
6. `7451b6f9`

### 推荐执行方式

不要在主分支直接连 pick，先建验证分支：

```bash
git switch -c backport/openai-stability-wave
git cherry-pick -x 6b646b61
git cherry-pick -x 3a07e92b
git cherry-pick -x c5aac125
git cherry-pick -x 4fb16030
```

对于 `7eecc49c`、`7451b6f9`，建议优先**手工移植**而不是直接 `cherry-pick`，因为这两条更容易与当前 fork 的 OpenAI 定制语义发生冲突。

### 回收后重点验证

- OpenAI API token 账号是否仍按当前 fork 预期工作
- passthrough 的 429/529 是否能正确 failover
- 非 Codex 客户端的 session hash 是否更稳定
- 限流信号是否不会错误回写成永久不可用状态

## 5.2 主题 D：上游响应读取与大响应保护

### 候选提交

- `10699eeb` `refactor: extract ReadUpstreamResponseBody to deduplicate upstream response read + too-large error handling`

### 判断

这条提交本身价值是有的，它把上游响应体读取和超大响应保护逻辑抽成公共能力。问题是它同时触及：

- `backend/internal/service/gateway_service.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/gemini_messages_compat_service.go`

而这三个文件里，前两个正是 fork 的重度定制核心区。

### 建议

- **不建议直接 cherry-pick**
- 如果你们最近在排查“响应体过大、读取不一致、重复逻辑”问题，可以把它作为**手工参考 patch**逐块移植

## 6. 条件性回收主题

## 6.1 主题 E：OpenAI messages 调度映射与 instructions 模板

### 候选提交

- `23c4d592` `feat(group): 增加messages调度模型映射配置`
- `4de4823a` `feat(openai): 支持messages模型映射与instructions模板注入`
- `de9b9c9d` `feat(admin): 增加分组 messages 调度映射配置界面`
- `d765359f` `test(admin): 增加messages调度表单状态转换测试`
- `57d0f979` `fix(frontend): 补全 messages 调度国际化文案`

### 推荐理由

这组能力与当前 fork 的 OpenAI 接入方向高度相关，也比“整套渠道管理系统”更聚焦，属于值得回收的功能主题。

### 风险

- 中到高

原因：

- 需要数据库迁移：`091_add_group_messages_dispatch_model_config.sql`
- 需要更新 ent/schema 和 group 相关 service
- 会触及 `admin_service.go`、`openai_gateway_handler.go`、`group_handler.go`
- 当前 fork 的 OpenAI 自定义逻辑较多，注入模板和调度映射是否与现有行为兼容，需要实测

### 建议

如果业务明确需要这项能力，可以作为单独专题做，不建议夹在 Wave 1/2 里顺手回收。

### 推荐方式

新建专题分支，按以下顺序处理：

1. 先回收后端模型与迁移
2. 再回收 service/handler
3. 最后回收前端管理界面
4. 全链路联调后再合主分支

## 6.2 主题 F：表格后端排序、搜索与分页配置

### 候选提交

- `5f8e60a1` `feat(table): 表格排序与搜索改为后端处理`
- `ad80606a` `feat(settings): 增加全局表格分页配置,支持自定义`
- `66e15a54` `fix(export): 导出逻辑与当前筛选条件对齐`
- `d8fa38d5` `fix(account): 修复账号管理中的状态筛选`

### 推荐理由

如果你们后台数据量已经上来，这组能力的运营收益会比较直接。

### 风险

- 中到高

原因：

- 涉及的 handler、repository、admin API、前端页面非常多
- 会碰到当前 fork 已修改的：
  - `frontend/src/views/admin/AccountsView.vue`
  - `frontend/src/api/admin/accounts.ts`
  - `backend/internal/repository/account_repo.go`
  - `backend/internal/service/admin_service.go`

### 建议

- 如果当前主要痛点是后台列表慢、排序不准、导出不一致，这组值得做
- 否则可暂缓，避免在运维核心链路之外引入大面积非核心改动

## 6.3 主题 G：OIDC 登录

### 候选提交

- `02a66a01` `feat: support OIDC login.`
- `8e1a7bdf` `fix: fixed an issue where OIDC login consistently used a synthetic email address`

### 风险

- 中到高

原因：

- 同时碰后端配置、认证路由、前端登录页、设置页、公开配置 DTO
- `setting_handler.go` 已是 fork 与上游的重要冲突点

### 建议

- 如果近期明确有企业 SSO / OIDC 诉求，可以单独做
- 如果没有明确业务驱动，先不回收

## 7. 当前阶段不建议回收的大主题

## 7.1 主题 H：渠道管理与统一计费大改

### 代表提交

- `91c9b8d0`
- `2555951b`
- `0b1ce6be`
- `8d03c52e`
- `632035aa`
- `a51e0047`
- 以及 081~089 一系列迁移

### 不建议原因

- 这不是单一功能，而是一整套平台能力重构
- 与当前 fork 的 `gateway_service.go`、计费逻辑、管理界面都有大量重叠
- 直接回收极易把当前 fork 的 OpenAI/Claude 定制链路打乱

### 建议

- 如果未来真要对齐上游的平台化能力，应单独立项，不应作为“选择性回收”的一部分

## 7.2 主题 I：支付系统

### 代表提交

- `63d1860d` 及后续一整串 `payment` 提交

### 不建议原因

- 引入大量新 ent schema、handler、service、provider、前端页面、迁移
- 不是小范围回收，而是功能系统落地
- 会显著增加维护面和发布风险

### 建议

- 仅在你们明确要上线支付能力时，再作为独立项目做

## 7.3 主题 J：通知系统与 WebSearch 大包

### 代表提交

- `b32d1a2c` 及其 notify 系列提交
- `fda61b06`、`1b53ffca` 及其 websearch 系列提交

### 不建议原因

- 都会撞到 `setting_handler.go`
- WebSearch 还会碰 `gateway_service.go`
- 主题跨度大，回收后还要配套 UI、测试、配置项、迁移

### 建议

- 如果确有业务优先级，可以拆成两个独立专题，但不建议与 Wave 1/2 混做

## 8. 建议的实际执行方案

## 8.1 第一周建议落地内容

建议先完成以下内容：

1. 安全补丁
2. 运维与调度稳态修复
3. OpenAI 链路稳定性中的低到中风险提交

推荐组合：

```bash
git switch -c backport/wave1-safety-stability
git cherry-pick -x 7060596a
git cherry-pick -x 217b7ea6
git cherry-pick -x 6401dd7c
git cherry-pick -x 3944b3d2
git cherry-pick -x 5d586a9f
git cherry-pick -x 6b646b61
git cherry-pick -x 3a07e92b
git cherry-pick -x c5aac125
git cherry-pick -x 4fb16030
```

### 这批做完后的判断点

- 如果冲突量小、回归测试稳定，可以继续评估 `7eecc49c`
- 如果 `openai_gateway_service.go` 冲突已经很多，就不要继续推高风险批次，应先收口

## 8.2 第二周再评估的内容

视 Wave 1 结果再决定是否进入以下主题：

- `7eecc49c` / `7451b6f9` 这类 OpenAI 深水区修复
- messages 调度映射与 instructions 模板
- 后端排序与分页配置

## 9. 最终建议结论

当前最合理的回收策略不是“大而全”，而是：

- **立即回收**：安全补丁、运维稳态修复、OpenAI 低中风险稳定性补丁
- **按业务需要单独专题回收**：messages 调度映射、OIDC、表格后端排序
- **暂缓**：渠道管理大改、支付系统、通知/WebSearch 大包

一句话概括：

> 先把上游“修 bug、补安全、提稳定”的价值拿回来，再决定是否引入“会改变平台形态”的大能力。

