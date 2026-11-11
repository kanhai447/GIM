# 005 / Task 4 — 删除无调用方的 group-rpc

- **日期**：2026-11-11（Asia/Shanghai）
- **开始状态**：HEAD `83db97a`，working tree clean（`004` 日志尚未提交）。
- **结束状态**：HEAD `bcacbae`。

## 目标

删除已实现但没有任何调用方的 Group RPC 服务，并同步 Compose、镜像构建、环境模板与拓扑文档。

## 删除前复验（grep 证据）

```text
$ grep -rn "groupv1|GroupServiceClient|group-rpc|GROUP_RPC" server --include=*.go
server/api/group/v1/*             -> 生成代码自身（package groupv1）
server/internal/group/transport/grpc/server.go + server_test.go -> 仅服务端实现与自身测试
server/cmd/group-rpc/main.go      -> 仅启动该 gRPC server
```

- **没有** `NewGroupServiceClient` 的业务调用方；`group-api` 直接 `group.New(mysql, redis, userRPC, ...)`。
- `group-api` 的 `depends_on` 不包含 group-rpc，因此删除不影响启动依赖。
- 非 Go 引用仅出现在 `compose.yaml`、`docker/server.Dockerfile`、`.env.example`、`.env.compose.example`、README/docs 以及历史 `agent_logs/`（历史日志不改写）。

## 删除文件

```text
server/cmd/group-rpc/main.go
server/internal/group/transport/grpc/server.go
server/internal/group/transport/grpc/server_test.go
server/api/group/v1/group.pb.go
server/api/group/v1/group_grpc.pb.go
server/api/group/v1/group.proto
```

## 同步修改

| 文件 | 变更 |
|---|---|
| `compose.yaml` | 删除 `group-rpc` service 与其健康检查/依赖 |
| `docker/server.Dockerfile` | 删除 `gim-group-rpc` 构建行 |
| `.env.example` / `.env.compose.example` | 删除 `GROUP_RPC_PORT` / `GROUP_RPC_HOST` |
| `README.md` | 拓扑删除 `GR --> RR[Group RPC]`，说明当前只有 User RPC |
| `docs/01_PROJECT_ARCHITECTURE.md` | 目标拓扑改为“API + 唯一 User RPC”，补充删除原因 |
| `docs/05_HTTP_API.md` | Group 章节删除“内部 RPC/Checkpoint 未实现 Group WS”的过期描述，改为当前实现 |
| `docs/architecture/overview.md` | Group 依赖改为 MySQL/Redis/User RPC |
| `docs/deployment/docker-compose.md` | “User/Group RPC” → “User RPC” |
| `docs/interview/project-overview.md` | “User/Group 提供内部 RPC” → “User 提供内部 RPC” |

## 执行命令与结果

```powershell
go build ./...                    -> PASS
go vet ./...                      -> PASS
gofmt -l .                        -> 无输出
go test ./... -count=1            -> PASS（69 个包 ok，无 FAIL）
docker compose config --quiet     -> PASS
docker compose config --services  -> 18 个 service（原 19，group-rpc 已消失）
grep gim-group-rpc|9094|GROUP_RPC  -> 除历史 agent_logs 外无残留
```

## 未执行

- `docker compose build` / `up`：daemon 未运行，无法验证镜像构建与运行拓扑。
- 删除后的 Compose 全栈 smoke：同上。

## Git

- commit：`bcacbae refactor(group): remove the unused group-rpc service`
- push：**失败（本地 commit 保留）**。GitHub 连接在收尾阶段不可达（`Recv failure: Connection was reset` / `Could not connect to server`），重试两次未成功；未使用 `--force`。
- tag：无。

## 遗留

- `.env.example` 仍保留 `AUTH_RPC_PORT`、`CHAT_RPC_PORT`、`FILE_RPC_PORT`、`SETTINGS_RPC_PORT` 等未使用占位键（本轮只被授权删除 `GROUP_RPC_*`）。这些键不会被任何服务读取，建议后续单独清理。
- `docs/01` 第 1 节仍描述“原项目 User/Chat/Group/File/Settings 包含 API/RPC”，那是对 FIM 的事实描述，不是 GIM 当前拓扑，已用第 2 节的新说明区分。

## 下一步

全量验收 + 状态日志更新。
