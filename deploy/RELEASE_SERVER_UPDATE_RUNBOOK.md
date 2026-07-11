# Sub2API 发布 + 服务器升级 Runbook

> 给 LLM/维护者用的最短安全流程：**先推 GitHub tag 生成 Release assets，再让服务器跑 `install.sh upgrade`**。不要直接 scp/覆盖服务器二进制，避免版本、前端静态资源、checksums、install 脚本检测结果不一致。

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

按改动范围跑最小验证：

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
