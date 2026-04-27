# Claude Code 指纹策略小白指南

> 目标不是追求“永远不会被拦截”的神奇参数，而是让请求长期看起来稳定、连续、接近真实 Claude Code 链路。

## 最推荐配置

在后台进入「客户端 / 工具指纹策略」：

1. 点击「一键推荐」。
2. 保存设置。
3. OAuth 上游账号建议开启 TLS 指纹。
4. 重要账号绑定固定 TLS 模板，不建议频繁随机切换。
5. Claude Code 最低/最高版本默认留空，只有确认某个版本有兼容问题时再设置。

推荐状态应为：

| 项目 | 推荐值 | 原因 |
| --- | --- | --- |
| 统一客户端指纹 | 开启 | 减少共享 OAuth 账号在不同工具之间来回漂移 |
| metadata.user_id 透传 | 关闭 | 由网关生成稳定身份，适合小团队共用 |
| CCH 签名 | 开启 | 让 billing header 占位符更接近真实客户端行为 |
| TLS 指纹 | OAuth 账号开启 | 让 TLS ClientHello 更接近 Node.js / Claude Code |
| 版本限制 | 留空 | 避免误拒绝正常升级后的 Claude Code |

## 两层指纹要分开理解

### 1. HTTP 指纹：网关会自动学习

包括：

- `User-Agent`
- `X-Stainless-Lang`
- `X-Stainless-Package-Version`
- `X-Stainless-OS`
- `X-Stainless-Arch`
- `X-Stainless-Runtime`
- `X-Stainless-Runtime-Version`
- Claude Code session / metadata / billing header

做法：让真实 Claude Code 通过网关请求一次即可。网关会按上游账号缓存这组字段，后续同账号复用。
同时后台「客户端 / 工具指纹策略」会把真实 Claude Code 请求保存到「HTTP 指纹样本库」。
团队里只要有一个人成功跑过真实 Claude Code，管理员就可以把这组样本设为全局使用，其他小白用户不用再理解这些字段。

示例：

```bash
ANTHROPIC_BASE_URL="https://你的网关域名" \
ANTHROPIC_AUTH_TOKEN="sk-你的网关密钥" \
claude -p "ping"
```

如果你的客户端使用 `ANTHROPIC_API_KEY` 而不是 `ANTHROPIC_AUTH_TOKEN`，保持原客户端习惯即可；关键是让请求经过网关一次。

### 2. TLS 指纹：用采集器导入模板

TLS 指纹不是 HTTP Header，无法只靠填写请求头模拟。它来自 TLS 握手阶段的 ClientHello。

小白流程：

1. 打开 TLS 采集器：<https://tls.sub2api.org>
2. 用真实 Claude Code 所在机器/网络访问采集器并生成 YAML。
3. 后台「TLS 指纹模板」粘贴 YAML 并保存。
4. 在对应 OAuth 账号上启用 TLS 指纹，并绑定这个模板。

如果没有真实模板，内置默认模板也可作为兜底；只有遇到 Claude Code 客户端限制时，再优先采集真实模板。

## 排查顺序

遇到类似错误：

```json
{
  "error": {
    "message": "This model is restricted to Claude Code clients only and cannot be accessed through other API clients.",
    "type": "permission_error"
  },
  "type": "error"
}
```

按这个顺序查：

1. 「客户端 / 工具指纹策略」是否为稳定模式。
2. CCH 签名是否开启。
3. 「HTTP 指纹样本库」里是否已有真实样本；有的话先选一个最近成功的样本。
4. 是否用真实 Claude Code 通过网关预热过一次。
5. OAuth 账号是否启用了 TLS 指纹。
6. 重要账号是否绑定了固定 TLS 模板。
7. 是否设置了过严的 Claude Code 版本限制。

## 不建议这样做

- 长期关闭 CCH 签名。
- 频繁随机更换 TLS 模板。
- 对所有普通客户端强套 Claude Code 指纹。
- 不确认原因就设置 Claude Code 版本上限。
- 多个团队成员混用同一个下游 key，导致 session 行为混乱。
