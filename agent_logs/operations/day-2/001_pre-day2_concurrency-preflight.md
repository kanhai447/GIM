# Day 2 / Operation 001 — Pre-Day2 Concurrency Preflight

- **时间：** 2026-10-09 10:43 +08:00
- **状态：** COMPLETED
- **范围：** 只检查并发测试环境以及 Day 2 的 Schema、Redis、Auth/JWT、Gateway Upgrade 和 FIM 参考边界；未实现 Chat WebSocket、Hub、Client、readPump、writePump 或 Presence。

## 目标

- 在正式进入 Day 2 Chat WebSocket 前确认已有基础是否满足 Phase 2/3 的开发入口。
- 准确复现 Windows race detector 环境问题；只接受无需安装或大改工具链的小型安全修正。
- 确认当前分支与远端一致、工作区无业务改动，并把结论写入可审计日志。

## 开始状态

- Branch：`main`。
- 开始 HEAD：`a638581ab538e1ada77e123ad694dc00c888e98b`。
- `git fetch origin`：SUCCESS。
- `HEAD == origin/main`：PASS。
- Working tree：CLEAN。
- Day 1：COMPLETED；Day 2 业务代码：NOT STARTED。

## 读取范围

- 完整重读 `AGENTS.md`、`START_AGENT.md`、`FINAL_AGENT_INSTRUCTION.md`、`README.md` 和 `docs/00`–`docs/16`。
- 重读 `agent_logs/CURRENT_STATE.md`、`agent_logs/day-1.md`、`agent_logs/AGENT_WORKLOG.md` 与 Day 1 最终验收日志 `agent_logs/operations/day-1/010_checkpoint-9_day1-final-acceptance.md`。
- 只读检查 GIM migration、Redis client/key helper、Auth JWT/authentication、Gateway route/proxy 及 FIM Chat/Group WebSocket 参考实现。

## 1. Race Detector

### 环境

- Go：`go1.25.2 windows/amd64`。
- `GOOS=windows`，`GOARCH=amd64`，`CGO_ENABLED=1`。
- Go `CC=gcc`，`CXX=g++`。
- 实际 C 编译器：`D:\describe\dc\MinGW\bin\gcc.exe`，MinGW.org GCC 6.3.0；target 为 `mingw32`，配置架构为 `i586`。
- PATH 和常见本机位置未发现 `clang`、`x86_64-w64-mingw32-gcc`、MSYS2 mingw64/ucrt64 GCC 或其他已安装的 64 位 C 编译器。

### 实测

```text
cd server
go test -race ./...
```

结果：FAIL（环境构建失败，测试未开始）。准确根因：

```text
# runtime/cgo
cc1.exe: sorry, unimplemented: 64-bit mode not compiled in
```

当前 `gcc` 是 32 位 MinGW 编译器，无法为 `windows/amd64` 构建 race 所需的 cgo runtime。本机没有可通过仅切换 `CC` 使用的兼容 64 位编译器；继续处理需要安装/替换 Windows C 工具链或改用可用 CI/其他环境，超出本轮“小且安全环境修正”范围。

**Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT**。

Day 2 要求：普通并发测试不得省略；正式验收必须在兼容的 64 位 C 工具链或 CI 环境执行 `go test -race ./...`，但该本地环境限制不阻塞 Day 2 开发。

## 2. Chat Schema Readiness

`server/migrations/003_chat.up.sql` 已具备：

- `chat_messages`。
- 发送/接收语义字段为文档约定的 `send_user_id` / `rev_user_id`（分别对应本预检清单中的 sender/receiver），均为 `BIGINT UNSIGNED NOT NULL`。
- `client_msg_id VARCHAR(64) NOT NULL`。
- 唯一键 `ux_chat_messages_sender_client (send_user_id, client_msg_id)`，满足同一发送者的 clientMsgId 幂等约束。
- 双向会话生成列与游标索引，以及发送/接收方向索引。
- `chat_sessions`。
- 唯一键 `ux_chat_sessions_user_peer (user_id, peer_user_id)`。
- `last_message_id`、`last_message_at`、`unread_count`、`last_read_message_id`、`is_top` 全部存在；unread/top 默认值符合文档。

隐藏/删除语义也已准备：

- `chat_sessions.hidden_at` 是当前用户维度的会话隐藏状态，不代表删除消息或全局撤回。
- `chat_message_deletions` 以 `(user_id, message_id)` 为主键，仅表达某个用户隐藏某条历史；与撤回语义分离。

现有 migration integration test 已断言关键列、唯一索引、重复消息/会话写入和 session/history 索引。未发现与 `docs/04_DATABASE_DESIGN.md` 的小型 schema contract 不一致，也没有 Day 1 schema 遗漏。本轮未修改 migration。

## 3. Redis Readiness

- `internal/platform/redis.Client` 已基于 `go-redis/v9` 提供安全配置、连接池、context-aware health check、`Raw()` 访问和显式 `Close()`，可作为 Day 2 Redis 依赖。
- `internal/platform/rediskeys` 已建立集中 key helper 边界，当前只实现 Auth logout fingerprint key。
- Presence 和 connection/device tracking key 尚未实现，符合 Day 2 尚未开始的状态。
- 正式实现 Presence 时必须先在 `rediskeys` 中统一新增 `gim:presence:user:{uid}` 及确有需要的连接/设备 tracking helper；业务代码不得散落硬编码 key。
- 本轮未创建 Presence key、未写 Redis Presence 数据。

结论：Redis client READY；Presence key helper 是 Day 2 Hub/Client foundation 的实现事项，不是 blocker。

## 4. Auth / WS Handshake Readiness

- Auth JWT 已包含 `userID`、`role`、`jti`、`iat`、`exp`，限制 HS256，校验签名、过期时间、签发时间、ID、用户 ID 和角色。
- Gateway 的 `requestToken` 先读 `token` Header，再回退到 URL query `token`；已有测试覆盖 `/api/chat/ws/chat?token=...`。
- Upgrade 前 Gateway 继续调用 Auth `/api/auth/authentication`；Auth 验证 JWT 和 Redis logout blacklist，成功后 Gateway 删除外部伪造身份头并注入可信 `User-ID` / `Role`。
- Chat 服务不应复用/复制 Auth JWT parser，也不应把 Gateway 改成本地验签；正确复用边界是既有 Gateway `authclient` + Auth service。内部 Chat endpoint 依赖可信注入身份及网络隔离。
- V1 保持 `...?token=JWT`；未引入 WS ticket。

结论：JWT/WS handshake authentication boundary READY。本轮未建立 WebSocket。

## 5. Gateway Upgrade Readiness

- 显式 route 已将 `/api/chat/...` 映射到 `chat_api` discovery service。
- Gateway Upgrade 检测正确识别 `Connection` 中大小写不敏感、逗号分隔的 `upgrade` token，并要求非空 `Upgrade` Header。
- 对 Upgrade 请求不附加普通 HTTP proxy timeout，避免长连接被 `GATEWAY_PROXY_TIMEOUT` 主动取消。
- 标准库 `httputil.ReverseProxy` 保留路径/query/Header 并支持协议 Upgrade；Gateway 自身没有配置会固定截断 WS 生命周期的 `WriteTimeout`。
- focused tests `./internal/gateway/...` 全部 PASS，已有 `TestUpgradeDetection` 与 query token 测试。
- 当前尚无 Chat WS upstream，因此不能做真实端到端 WS tunnel 验收；该验收应在 Day 2 第一个业务 Checkpoint 建立最小 Chat endpoint 后补充。

结论：未发现 Gateway 明显无法代理 Upgrade 的前置缺陷，无需本轮修复。

## 6. FIM Chat WebSocket 只读参考结论

只读检查：

- `reference/fim_server-main/fim_server-main/fim_chat/chat_api/internal/handler/chathandler.go`
- `reference/fim_server-main/fim_server-main/fim_group/group_api/internal/handler/groupchathandler.go`

确认原实现问题：

1. Chat/Group 分别使用包级普通 `map[uint]*UserWsInfo` 和每用户普通连接 map，没有 Hub 单所有者或互斥保护；连接注册、删除、遍历和业务发送可并发访问。
2. handler、错误响应、好友通知、私聊推送和群广播等多个路径直接调用 `Conn.WriteMessage` / `Conn.WriteJSON`；同一连接可能被并发写入，没有统一 writePump。
3. 没有 Gorilla Ping/Pong control frame、read deadline、PongHandler 或心跳超时清理。
4. Chat/Group `CheckOrigin` 均无条件 `return true`。
5. 多设备使用 remote address 作为连接 key；Chat 每次新连接都会 `HSet online` 并执行好友上线通知，没有严格限制为连接数 `0 -> 1`，因此新标签页可能重复触发上线语义。
6. `CurrentConn` 会被最后连接覆盖；部分发送遍历全部连接，部分接收方发送在第一个连接后 `break`，多设备投递语义不一致。
7. Chat 在本地连接数归零时删除 Redis online，方向与 GIM 目标接近，但普通 map 无并发保护，注册/注销计数和 Presence 更新不是安全的串行状态机。
8. Group 仅维护自己的本地连接 map、不写 Redis online；GIM 保留“Presence 只由 Chat 管理”的边界，但必须用独立 Group Hub 安全实现本地连接管理。

本轮未复制任何 FIM 代码。GIM 后续将依据自身文档自主实现 Hub/Client、单 writePump、Ping/Pong、配置化 Origin 与 0→1/1→0 多端 Presence。

## 测试

- `go test -race ./...` -> NOT AVAILABLE；在 runtime/cgo 构建阶段因 32 位 MinGW 不支持 64 位模式失败。
- `go test ./internal/auth/token ./internal/auth/service ./internal/gateway/... ./internal/platform/redis ./internal/platform/rediskeys -count=1` -> PASS。
- `go test ./... -count=1` -> PASS。
- 没有修改业务代码、DB schema、HTTP/WS 协议或 Redis 数据。

## 修改文件

- `agent_logs/operations/day-2/001_pre-day2_concurrency-preflight.md`
- `agent_logs/CURRENT_STATE.md`
- `agent_logs/AGENT_WORKLOG.md`
- `agent_logs/day-2.md`

## Blocker / 下一步

- Day 2 blocker：NONE。
- 本机 race 环境是验收环境缺口，不阻塞开发；兼容环境/CI race 仍是 Day 2 完成门禁。
- Next Checkpoint：Chat WebSocket Hub / Client foundation。
- 下一 Checkpoint 首批实现事项：加入 Gorilla WebSocket 依赖、集中定义 Presence/可选 connection tracking key、建立 Chat Hub/Client 和 Upgrade endpoint，并补真实 Gateway WS tunnel 测试。不得在本预检中提前实施。

## Git

- Commit：本操作日志所在的 `chore(day2): record concurrency preflight` 提交。
- Push：完成 Secret Scan 和 staged diff 复核后推送 `origin/main`；实际结果以本轮最终 Git 复核为准。
- Tag：无；Pre-Day2 preflight 不代表 Day 2 业务完成。
