# 003 / Task 3 — 代码整洁

- **日期**：2026-11-12（Asia/Shanghai）
- **开始状态**：`HEAD = 830956d`。
- **结束状态**：本任务提交 `chore: remove dead code and unused config`。

## 目标

删除有 grep 证据的孤儿包/死组件/无调用方构造函数，清理无读取的配置键与过期 checkpoint/Day 文案；保留业务实现与测试，删除后 build/vet/test 与前端检查全部通过。

## 5.1 孤儿包与死代码

| 对象 | grep 证据 | 处理 |
|---|---|---|
| `server/internal/project/` | `internal/project` 在 `server/**/*.go` 中 **0 处 import**；包内只有 `doc.go`（注释），`go list -deps ./cmd/...` 不可达 | **删除整个包** |
| `web/src/components/FeaturePlaceholder.vue` | 组件名在 `web/src/**/*.{ts,vue}` 中除自身外 **0 处引用** | **删除** |
| `admin/src/components/ModulePlaceholder.vue` | 组件名在 `admin/src/**/*.{ts,vue}` 中除自身外 **0 处引用** | **删除** |
| `chat.New(values, ...)` | `\bchat\.New\(` 全仓 **0 处调用**；生产入口使用 `chat.NewWithInboundFactory` | **删除该导出函数**（`UnavailableInboundHandler` 保留，测试仍在使用） |

保留：`server/api/user/v1`（Auth/Chat/Group 依赖 User RPC）；`internal/chat.UnavailableInboundHandler`（测试替身）。

完整导出函数级死代码扫描没有现成工具接入，已登记为 `docs/known-issues.md` E-7，不作为本轮阻塞。

## 5.2 未使用配置键

逐键检查两个模板（代码字面量 + 动态键构造 `prefix+"_API_PORT"` / `prefix+"_RPC_PORT"` / `*_API_HOST` / `*_API_INSTANCE_ID` 也算已使用）：

删除（代码/compose/scripts 均 0 读取）：

```text
APP_ENV
LOG_LEVEL
WEB_ORIGIN
ADMIN_ORIGIN
```

保留：`MYSQL_ROOT_PASSWORD`（Compose healthcheck 使用）、`INTERNAL_API_HOST` / `INTERNAL_BIND_HOST`、各 `*_API_*`、`USER_RPC_*`、Kafka/WS/File 键。

## 5.3 过期注释与 UI 文案

grep `TODO|FIXME|later checkpoint|added by later|remain later|later work|Day [0-9]`（`server/`、`web/src/`、`admin/src/`）：

| 位置 | 内容 | 处理 |
|---|---|---|
| `server/internal/project/doc.go` | “added by later checkpoints” | 随孤儿包删除 |
| `web/src/views/HomeView.vue` | “Day 1 foundation”、“后续 Checkpoint”、“待 Day 2/3” | 改写为当前 V1 能力（私聊/群聊/好友/文件/审计） |
| `web/src/components/FeaturePlaceholder.vue` | “后续 Checkpoint”标签 | 随死组件删除 |
| `admin/src/layouts/AdminLayout.vue` | “Day 4 · Admin Integration” | 改为 “GIM V1 · Admin” |

结果：代码与 UI 中无剩余 `TODO/FIXME`、无 checkpoint/later work 文案。

## 问题与修复

- 最初用字面量 grep 扫描配置键，`*_API_PORT`/`*_RPC_PORT` 等动态构造键被误报为“无读取”；改为同时识别 `prefix+"_API_PORT"`、`prefix+"_RPC_PORT"`、`*_API_HOST`/`*_API_INSTANCE_ID` 后重新确认，只删除真正无读取的 4 个键。
- 未使用导出函数没有自动扫描工具；`chat.New` 经全仓调用点 grep 确认无调用后删除，其余导出符号的自动扫描登记为 `docs/known-issues.md` E-7。
- 未发现业务缺陷；本任务未改业务逻辑。

## 验证

```powershell
# 后端
go build ./...                 -> PASS
go vet ./...                   -> PASS
gofmt -l .                     -> 无输出
go test ./... -count=1         -> PASS（69 个包 ok）
docker compose config --quiet  -> PASS

# 前端
cd web;   vue-tsc -> PASS; vitest 19 files / 103 tests -> PASS; vite build -> PASS
cd admin; vue-tsc -> PASS; vitest 11 files / 25 tests -> PASS; vite build -> PASS
```

## 修改文件

- 删除：`server/internal/project/doc.go`、`web/src/components/FeaturePlaceholder.vue`、`admin/src/components/ModulePlaceholder.vue`
- 修改：`server/internal/chat/module.go`、`web/src/views/HomeView.vue`、`admin/src/layouts/AdminLayout.vue`、`.env.example`、`.env.compose.example`、`docs/known-issues.md`

## Git

- commit：`chore: remove dead code and unused config`（本记录所在提交）。
- push：随本轮各任务提交推送。

## 下一步

Task 4：agent_logs 结构统一。
