# Sub2API 发布 + 服务器升级 Runbook

> 给 LLM/维护者用的最短安全流程：**先推 GitHub tag 生成 Release assets，再让服务器跑 `install.sh upgrade`**。不要直接 scp/覆盖服务器二进制，避免版本、前端静态资源、checksums、install 脚本检测结果不一致。

## 当前生产保留门禁（2026-10-17）

**用户明确要求：当前生产可用，保留现有功能、配置和备份，不因规范发布而删除或回滚。下次发布前先执行本节；后文模板不能跳过本节。**

### 已核实的生产基线

截至 **2026-10-17 05:00，Asia/Shanghai**：

| 项目 | 已核实状态 |
| --- | --- |
| 服务 | `sub2api`，`active/running`，`NRestarts=0`，本机 `/health=200` |
| 运行文件 | `/opt/sub2api/sub2api`；磁盘与 `/proc/<MainPID>/exe` 的 SHA-256 一致 |
| 当前文件 SHA-256 | `2894ccb3e0953c5858227277b86b3108ac248e8f2618358fc7c90e74d21ceb25` |
| 自报版本 | `0.1.325`，`commit/built=unknown`，`build_type=source`；**不是正式 tag 的同一文件** |
| 正式旧版备份 | `/opt/sub2api/backups/sub2api-jb-pre-20261017044053`；SHA-256：`b50e9a75f31756753e55884f1a6562dbd8f94bd8b76385d48a69a12cb9e6fa4b` |
| 数据库 | `153_add_group_jailbreak_enabled.sql` 已执行；按迁移器 `TrimSpace` 后 SHA-256 为 `49a87f8f969a7e77b3ac655d6f8af2000121f554f97de7cec7caff376bff0aed`，与库记录一致 |
| 对外入口 | Caddy 磁盘配置和运行配置均为 `ib.do → 127.0.0.1:59012`；不要按未经核实的 `8080` 描述改端口 |

这些值是历史快照，不是自动恢复目标。每次操作必须重新读取；不一致时先查后续部署记录，不能强制恢复成表中的旧状态。

### 下次更新的必做检查

1. **先保全，再发布。** 核对当前运行文件、版本、配置和数据库迁移；为当时正在运行的文件另建带时间戳的备份并记录 hash。保护已有备份，不复用旧文件名覆盖；有数据库写入或迁移时另留数据库/配置恢复点。
2. **不能从旧 tag 单独构建一个小补丁就覆盖生产。** 当前线上包含本地尚未提交的功能；先盘点工作区已修改及未跟踪文件，确认部署所需源码、迁移、生成代码和前端来源已完整归档并进入对应发布提交。不能认定“现在的工作区”自动等于已部署源码；来源不明时先补齐记录，不继续替换。保留 `.omo/` 和其他任务改动，不使用 `git clean`、`reset --hard` 或覆盖式 checkout 清理现场。
3. **发布一个未使用的新版本。** 检查已有 tag 后再定版本号，不重用或覆盖 `v0.1.325`。构建须注入真实 `Commit`、`Date`、`BuildType=release`，前端与后端来自同一发布提交；发布说明列明保留的生产定制和本次变化。不能重新安装正式 `v0.1.325` 并把它当作当前生产的等价文件。
4. **已执行迁移不可再改。** 保留 `153` 的文件内容与校验；后续变更使用新的唯一迁移编号。回退二进制不等于回退数据库，不能顺手删字段、删设置或执行降级 SQL。
5. **先确认发布包可用，后停止服务。** 按本手册生成 Release、校验目标架构和 checksum，再执行指定版本的 `install.sh upgrade`。Actions 不可用时执行前文已约定的用户授权流程，不偷偷回退到直接覆盖二进制。
6. **按页面与 API 验收，不只看 health。** 核对运行文件 hash、版本、管理员读取接口及既有协议的非破坏性探活；在浏览器实际打开 `/admin/settings` 和 `/admin/groups`，检查 JS/CSS 类型与加载错误。已有旧资源返回 `200 text/html` 的风险，须单独修复或明确记录；不能把强刷后能打开当作永久修复。
7. **回滚也先停服务。** 必须回滚时，先停止服务，再恢复经过校验的目标备份，最后启动并有限重试健康检查；不能用“先 `cp` 覆盖正在执行的文件，再 restart”的顺序，该顺序已在临时进程上复现 `Text file busy`。不要为演练回滚中断当前可用生产。

本节只记录发布保护要求：没有替换生产文件、调整分组/策略、修改 Caddy、删除备份或执行回滚。源码归档和下一版本发布仍须在实际更新时完成，不能仅凭本节宣称已实现可复现构建。

## 0. 固定变量

```bash
REPO=kw0ngr/sub2api
NEXT=0.1.258                 # 改成目标版本，不带 v
TAG=v${NEXT}
SERVER=root@168.222.0.214    # 改成目标服务器
```

如需密码登录：

```bash
export SSHPASS='***'
SSH="sshpass -e ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/tmp/sub2api_known_hosts"
```

如已配置 SSH key：

```bash
SSH="ssh -o StrictHostKeyChecking=no"
```

## 1. 发布前本地检查

```bash
git status --short --branch
git diff --check
```

确认：

- 不提交 `.omo/`、临时日志、服务器密码、截图草稿。
- 没有未完成的半截改动。
- 版本号只改：`backend/cmd/server/VERSION`。

**新增 OpenAI 模型时（目录/价格更新不等于账号已能调用）：**

1. 对目标分组的候选账号逐个用其真实 API Key 直连上游 `POST /v1/responses`，仅对 `HTTP 200 + status=completed` 的账号增加 `credentials.model_mapping`。`GET /models` 成功不是推理探活；`403 model_not_found` 只跳过该模型，不应禁用整个账号。批量改动前备份映射，完成后刷新调度快照。
2. 用**该分组的用户 Key**跑发布门禁（含流式请求，任一步 404/失败都不能宣称上线完成）：

   ```bash
   export SUB2API_BASE_URL='https://ib.do' SUB2API_API_KEY='<从密钥管理器获取>'
   python3 deploy/check_openai_model_routes.py gpt-6.1-sol gpt-6-sol gpt-6-luna
   unset SUB2API_API_KEY
   ```

   `chat-latest` 使用独立定价和能力描述：实测仅接受 `reasoning.effort=medium`，不支持 `max/none` 或采样参数。门禁对它省略 reasoning，使用上游默认；不可套用 GPT Sol 的 `max` 规则。

   每个实际使用该模型的分组都要单独验收。没有通过上游推理探活的账号，不要因旧型号可用就盲目映射新型号。

按改动范围跑最小验证：

- Grok 的 `403 permission-denied` 必须结合原文分类：`I'm sorry, I can't help with that request.` 是本次内容拒绝，不能停调账号/模型，也不能换号重试。Chat、Responses、透传和媒体入口须保留拒绝响应；真正的 chat 权限拒绝、失效凭证和额度不足仍按原规则处理。恢复存量账号前须核对冷却原因并用真实推理验收，不能批量清除所有 403 状态。
- Gemini 新模型须用真实 `generateContent` 验证；`/models` 成功不等于有对应模型额度。健康检查默认使用 `gemini-3.5-flash-lite`，避免新项目无 Gemini 2.5 权限被误判。Nano Banana 图像探活须实际收到非空 `inlineData`，不能以仅文本的 200 判定成功。
- Gemini 4 Argon（`gemini-4-argon`，2026-10-13 GA）已验证原生/OpenAI 文本调用；支持 low/medium/high，客户端 max/xhigh 仅对该 Gemini 模型转换为 high，不影响 GPT Sol。原生采样参数实测可用，不沿用 3.8 Flash 的采样删除规则。仅在真实推理成功的账号上加映射；免费层/Google AI Pro 计划不默认拥有权限。标准输入/缓存/输出价为 $2/$0.10/$10 每百万 token，2026-12-01 UTC 自动调整为 $4/$0.20/$20；Priority 为标准价的 1.8 倍。来源：[发布记录](https://ai.google.dev/gemini-api/docs/changelog)、[模型说明](https://ai.google.dev/gemini-api/docs/models/gemini-4-argon)、[定价](https://ai.google.dev/gemini-api/docs/pricing)。单请求同步输出上限 65,536；超过它需要上游 Interactions/background 长解码，本轮未增加该协议。
- Google OpenAI 错误可能包装为 `[{"error":...}]`，需解包后读取 QuotaFailure/RetryInfo；单模型 429 不应停调整账号。503 `This model is currently experiencing high demand` 仅短暂冷却目标模型。多轮工具调用必须由客户端回传原样的 `tool_calls[].extra_content.google.thought_signature`；网关不得伪造签名或把缺签名的 400 当失效 Key。
- Google OpenAI 也可能仅返回 `400 Please pass a valid API key`（省略 `API_KEY_INVALID` reason）。须精确识别此短句并摘除失效 Key，在提交响应前切换其他账号；普通参数/工具签名 400 不换号、不停调。2026-10-20 的账号 `1808` 已由原生接口明确复现 `API_KEY_INVALID`，保留备份并停调，不因 `/models=200` 或历史 active 状态恢复。
- DeepSeek 当前 Flash ID 是 `deepseek-flash`（V4.1）；旧 `deepseek-v4-flash` / `deepseek-v4-flash-vision-exp` 保留兼容，并使用同一 Flash 价格。默认价格采用官方峰时基准，未自动应用上游按时段/节假日变化的折扣；渠道自定义价格仍优先。
- DeepSeek V4.1 的客户端显示名可使用 `deepseek-v4.1-flash`，官方 API 仍须发送 `deepseek-flash`。只对官方 `api.deepseek.com` 归一化旧/新 Flash 别名；第三方渠道须实测其原生模型 ID，不套用官方 ID。兼容渠道的 OpenAI base URL 常含 `/v1`，Anthropic 测试拼接 `/v1/messages` 前须去除末尾 `/v1`，避免 `/v1/v1/messages`。账户 `3210` 的 HubCore 入口使用 `https://api.hubcore.cn/v1`，Flash 别名映射到该上游原生 `deepseek-v4.1-flash`；根路径返回 HTML 200 不算推理成功。
- DeepSeek 回归须覆盖 Chat/Responses 的 402：账号停调后返回未提交响应的换号错误；命名或 `required` 工具选择须复用原生 Messages 的思考兼容处理，普通 `auto` 调用仍保留 `max` 和思考回传。
- Gemini 3.6–3.8 Flash 的本地促销价格截止 2026-12-31，须在 2027-01-01 前重新核实官方价格。
- GLM `1113` 表示所选上游入口没有可用资源，不是普通短限流；只暂停该协议账号，不能联动禁用相同 Key 的其他协议账号。Coding Plan 的 OpenAI base URL 使用 `https://open.bigmodel.cn/api/coding/paas/v4`，须真实推理通过后才改配置和恢复。`/models=200` 不能单独恢复调度；本地等待队列满须记录为网关 routing 错误，不能填写伪造的上游 429。原始业务码保存在错误详情 `provider_error_code`。
- 原始 Key 批量导入勾选验证时，新账号须在入库时即不可调度；仅验证成功后恢复。健康任务冲突、网络错误、权限或限额未解决时保持停调，不能把 `created` 当成 `valid`。短十六进制 `sk-` Key 须明确 `platform,key` 或 `key,base_url`，`# deepseek` 等注释不会设置平台。OpenRouter/DeepSeek 余额只能作为辅助信号，须真实 Chat 完成；正余额小于 1 不应误禁用，DeepSeek 优先遵循 `is_available`。Grok 验证复用原生 Responses 探活及 CLI 头，须收到完成事件。发布前保存尚在内存的健康任务结果，待任务完成再升级；导入与目标分组绑定是两件事，未匹配 `platform-default` 时保留未分组，不擅自改原有生产分组。

```bash
cd backend
go test ./internal/pkg/xai ./internal/service ./internal/handler/admin -run 'Test.*Grok|Test.*OpenAI|Test.*Account|Test.*Usage' -count=1

cd ../frontend
pnpm exec vue-tsc --noEmit --pretty false
```

如果全量测试有既有失败，要记录失败点；不要把既有失败当作本次发布阻塞，除非触及本次改动路径。

## 2. bump 版本

```bash
printf '%s\n' "$NEXT" > backend/cmd/server/VERSION
git diff -- backend/cmd/server/VERSION
```

## 3. 提交与打 tag

```bash
git add backend frontend deploy .github .goreleaser.yaml .goreleaser.simple.yaml
git reset .omo 2>/dev/null || true

git status --short
git commit -m "release: self-use ${NEXT}"

git tag -a "$TAG" -m "release: self-use ${NEXT}" -m "- 修复/对齐本轮变更。"
```

只提交实际需要的路径；如果只改后端/文档，就不要 `git add frontend`。

## 4. 推送 main + tag

```bash
git push origin main
git push origin "$TAG"
```

GitHub Actions 的 Release workflow 由 `v*` tag 触发，会生成二进制 assets 和 `checksums.txt`。

## 5. 等 Release assets 生成完成

安装脚本会下载类似：

```text
sub2api_${NEXT}_linux_amd64.tar.gz
checksums.txt
```

用 GitHub API 检查：

```bash
until curl -fsSL "https://api.github.com/repos/${REPO}/releases/tags/${TAG}" \
  | grep -q "sub2api_${NEXT}_linux_amd64.tar.gz"; do
  echo "waiting release asset ${TAG} ..."
  sleep 15
done
```

也可以看 Actions：

```bash
gh run list --repo "$REPO" --workflow Release --limit 5
gh release view "$TAG" --repo "$REPO"
```

不要在 assets 还没出来时升级服务器；否则 install 脚本可能检测到 main 版本已更新，但下载不到 release asset。

Actions 被禁用时，只有用户明确同意后才可手动生成 Release：从干净 tag 导出源码，先构建前端，再按 `.goreleaser.yaml` 的 `CGO_ENABLED=0`、`-tags=embed` 和 commit/date/release ldflags 构建目标平台；上传同名 archive 和 `checksums.txt` 到 GitHub Release，核验资产后仍执行下节安装脚本。发布说明必须标明未构建的平台/Docker 镜像；禁止夹带工作区其他并行改动。

## 6. 服务器升级：只用 GitHub 上的 install.sh

优先使用 GitHub Contents API，绕过部分机器访问 `raw.githubusercontent.com` 404/超时的问题：

```bash
$SSH "$SERVER" '
set -e
curl -4 -fsSL \
  -H "Accept: application/vnd.github.raw" \
  --connect-timeout 10 --max-time 60 \
  "https://api.github.com/repos/kw0ngr/sub2api/contents/deploy/install.sh?ref=main" \
  -o /tmp/sub2api-install.sh
printf "1\n" | bash /tmp/sub2api-install.sh upgrade
'
```

升级指定版本：

```bash
$SSH "$SERVER" "
printf '1\n' | bash /tmp/sub2api-install.sh upgrade -v ${TAG}
"
```

## 7. 升级后校验

```bash
$SSH "$SERVER" '
set -e
sub2api --version || /opt/sub2api/sub2api --version
systemctl status sub2api --no-pager -l | sed -n "1,80p"
journalctl -u sub2api -n 120 --no-pager
'
```

至少确认：

- `--version` 显示目标版本。
- `systemctl status` 为 `active (running)`。
- 最近日志没有启动失败、数据库迁移失败、端口占用、panic。
- 管理端前端有变更时，浏览器强刷或清缓存确认 UI 已更新。

## 8. 失败处理

### 8.1 install.sh 显示已经是旧版本

检查 main 版本文件是否已推上去：

```bash
curl -fsSL \
  -H "Accept: application/vnd.github.raw" \
  "https://api.github.com/repos/${REPO}/contents/backend/cmd/server/VERSION?ref=main"
```

如果还是旧版本：说明 `git push origin main` 没成功或版本文件没提交。

### 8.2 main 是新版本，但下载 404

通常是 tag 推了，但 Release assets 还没生成完。回到第 5 步等待。

### 8.3 服务启动失败

```bash
$SSH "$SERVER" '
journalctl -u sub2api -n 300 --no-pager
systemctl status sub2api --no-pager -l
'
```

先看日志，不要直接覆盖二进制。需要回滚时：

```bash
$SSH "$SERVER" '
curl -4 -fsSL -H "Accept: application/vnd.github.raw" \
  "https://api.github.com/repos/kw0ngr/sub2api/contents/deploy/install.sh?ref=main" \
  -o /tmp/sub2api-install.sh
printf "1\n" | bash /tmp/sub2api-install.sh upgrade -v v0.1.257
'
```

把 `v0.1.257` 换成上一个已知可用 tag。

## 9. 禁止事项

- 禁止直接 scp 本地二进制覆盖服务器常规发布。
- 禁止只 push main 不 push tag。
- 禁止 tag 后不等 Release assets 就升级。
- 禁止提交 `.omo/`、密码、临时日志。
- 禁止发布前跳过 `git diff --check`。

## 10. 一条龙模板

```bash
REPO=kw0ngr/sub2api
NEXT=0.1.258
TAG=v${NEXT}
SERVER=root@168.222.0.214
export SSHPASS='***'
SSH="sshpass -e ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/tmp/sub2api_known_hosts"

git status --short --branch
git diff --check
printf '%s\n' "$NEXT" > backend/cmd/server/VERSION
git add backend/cmd/server/VERSION <本次实际改动文件>
git reset .omo 2>/dev/null || true
git commit -m "release: self-use ${NEXT}"
git tag -a "$TAG" -m "release: self-use ${NEXT}"
git push origin main
git push origin "$TAG"

until curl -fsSL "https://api.github.com/repos/${REPO}/releases/tags/${TAG}" \
  | grep -q "sub2api_${NEXT}_linux_amd64.tar.gz"; do
  echo "waiting release asset ${TAG} ..."
  sleep 15
done

$SSH "$SERVER" '
set -e
curl -4 -fsSL -H "Accept: application/vnd.github.raw" --connect-timeout 10 --max-time 60 \
  "https://api.github.com/repos/kw0ngr/sub2api/contents/deploy/install.sh?ref=main" \
  -o /tmp/sub2api-install.sh
printf "1\n" | bash /tmp/sub2api-install.sh upgrade
sub2api --version || /opt/sub2api/sub2api --version
systemctl status sub2api --no-pager -l | sed -n "1,80p"
'
```
