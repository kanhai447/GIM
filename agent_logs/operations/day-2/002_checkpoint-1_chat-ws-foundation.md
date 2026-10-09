# Day 2 / Operation 002 — Chat WebSocket Hub / Client Foundation

- **时间：** 2026-10-09 18:49 +08:00
- **状态：** COMPLETED
- **范围：** 仅建立 Chat WebSocket 安全并发基础、最小可运行 endpoint、Chat API runtime 和真实 Gateway tunnel；未实现 Heartbeat、Presence、ACK、消息持久化、session/unread 或前端 WebSocket。

## 开始状态

- Branch：`main`。
- 开始 HEAD：`36e458d0bd9f43c9492dc624877d5a66ef91982c`。
- `git fetch origin`：SUCCESS。
- 开始时 `HEAD == origin/main`：PASS。
- 开始时业务工作区：CLEAN。
- 检查到与本任务无关的未跟踪 `.tmp/sep_quality_revision/` 临时产物；未读取业务内容、未删除或改写，补充 `.tmp/` ignore 以防其进入 GIM 提交。

## FIM reference

只读参考 `reference/fim_server-main` 中 Chat WebSocket handler 的 endpoint、用户连接管理和消息流向语义；未复制文件、Hub/connection 实现或将 reference 作为依赖。

### FIM problems

- package 级普通全局 map 被多个 goroutine 读写。
- handler/业务路径可直接并发调用 `Conn.WriteMessage` / `WriteJSON`。
- 没有统一 writePump，也没有清晰的 Client 生命周期所有者。
- 没有 Ping/Pong；该问题已确认，但 Heartbeat 按范围留到下一 Checkpoint。
- `CheckOrigin` 无条件放行。
- 单用户多设备/多 tab 的投递、当前连接与上线语义不一致。

### GIM reimplemented → GIM improved

- GIM 自主实现 `Hub` event loop；连接 map 只在 Hub goroutine 内创建和访问，没有 package 全局 clients map。
- 数据结构为 `userID -> clientID -> Client`，同一用户的多个连接互不覆盖。
- `Register` / `Unregister` 为同步命令；重复注销安全，移除最后连接时删除用户空结构。
- Client 使用 128-bit CSPRNG 随机值编码为不含身份/Token 的 opaque ClientID。
- 每个 Client 使用有界 Send channel；队列满时 Hub 非阻塞地移除并关闭该 slow client，不影响其他用户/连接，也不创建无限 goroutine。
- `readPump` 只负责读取 text frame、解析 typed envelope、交给 `InboundHandler` 边界和注销；正式模块注入明确返回 unavailable 的 handler，没有伪装消息业务已完成。
- `writePump` 是生产代码唯一调用 `Conn.WriteMessage` 的数据写路径；Hub、handler、readPump 和未来业务只能经 Hub → Send → writePump。
- Hub shutdown 关闭所有连接和 Send channel；read/write pump 的重复注销安全，进程 shutdown 有超时，不无限等待。

## Protocol / endpoint / direct access

- endpoint：`GET /api/chat/ws/chat`；客户端继续使用 V1 `?token=JWT`，query token 只由 Gateway/Auth 在 Upgrade 前校验。
- Chat 不解析 JWT，只接受 Gateway 注入的 `User-ID` 和 `Role`（角色 1/2）；身份缺失/非法时在 Upgrade 前返回 401。
- 正式部署必须仅在内部网络暴露 Chat API，不能把公网客户端可伪造的身份 Header 当安全边界。本 Checkpoint 未重构既定 Auth 架构。
- typed envelope 复用文档字段：`event`、`requestId`、`clientMsgId`、`data`；仅建立严格解码边界，没有实现 clientMsgId 处理、ACK 或任何事件业务。
- frame/envelope 最大 1 MiB；只接受 text frame。

## Origin / security

- 新增配置 `CHAT_WS_ALLOWED_ORIGINS`，只接受精确 canonical `http` / `https` origin。
- 缺失、重复、非法、带 path/query/fragment 或不在 allowlist 的 Origin 均拒绝；没有 `CheckOrigin: true`。
- 开发模板显式允许 `http://localhost:5173` 与 `http://127.0.0.1:5173`；生产必须配置实际 allowlist。
- `CHAT_WS_SEND_BUFFER` 可配置且限制为 1–4096；默认 64。
- Secret Scan / repository safety：PASS；没有提交 `.env.local`、JWT、数据库/Redis凭据或 reference 内容。

## Chat runtime / etcd

- 新增正式 `server/cmd/chat-api`，复用 Day 1 process/env、go-zero REST、etcd client 和 discovery Registry。
- 注册 service `chat_api`，使用既有 lease/keepalive/cleanup；没有第二套 discovery。
- 正式编译的 Chat binary 启动后，etcd key `/gim/services/chat_api/chat-api-local` 可见；未带可信身份直连 endpoint 返回 401。
- 对正式 binary 发送 Ctrl+C 后，Chat 端口立即关闭且 service key 数量为 0，证明 REST、Hub、连接和 registration cleanup 路径可退出。PTY 包装器显示的进程码不作为服务退出码结论。

## Gateway real WebSocket tunnel

集成测试使用真实临时 etcd、真实 Auth handler、真实 Gateway reverse proxy/discovery，以及真实 go-zero Chat REST route/server：

- `WebSocket client -> Gateway -> Auth -> Gateway Upgrade -> Chat API`：PASS。
- 有效 JWT：101 Switching Protocols，Hub 注册可信 userID：PASS。
- 缺失 JWT：401，Upgrade 被拒绝：PASS。
- 经 Gateway Logout 后复用 JWT：401，Upgrade 被拒绝：PASS。
- 关闭连接后 Hub connection count 回到 0：PASS。
- 首次执行时本地 etcd 未启动而连接被拒绝；这是外部测试依赖未运行，不是代码失败。启动忽略目录中的临时单节点 etcd 后测试通过，并在测试后关闭 etcd。

## Tests

- `go test ./... -count=1`：PASS。
- `go vet ./...`：PASS。
- `gofmt -l .`：PASS（无输出）。
- `git diff --check`：PASS。
- `go test ./internal/chat/... -coverprofile=... -count=1`：PASS。
- Chat foundation 汇总 coverage：**78.6% statements**；`internal/chat/config` 100%，`internal/chat/protocol` 100%。
- `go test ./tests -run TestGatewayAuthChatWebSocketTunnel -count=1 -v`（真实临时 etcd）：PASS。
- Hub 单/多客户端、多用户、重复注销、最后连接清理、shutdown：PASS。
- 并发逻辑发送只产生单 writer、fake connection 未观察到并发 WriteMessage：PASS。
- slow-client 满队列不阻塞 Hub且不影响健康连接：PASS。
- Chat endpoint 双连接、关闭其一保留另一连接、最小 typed frame 出站、断开清理：PASS。
- allowed Origin 成功，disallowed/missing Origin 拒绝：PASS。
- `scripts/check-repository-safety.ps1`：PASS。

## Code review audit

- `WriteMessage(`：生产代码只有 socket interface 声明和 `writePump` 内调用；其余命中均为测试客户端/fake socket。
- `WriteJSON(` / `NextWriter(` / `WriteControl(`：生产 Chat 代码无命中。
- `CheckOrigin`：只绑定配置驱动的 `OriginPolicy.Allows`。
- global map：没有全局 clients map；Hub map 仅归 event loop 所有。
- `context.Background()`：生产端仅用于进程 lifecycle/startup/shutdown 和 pump 退出时的 best-effort 幂等注销；没有替代业务请求 context。
- `go func`：Chat runtime 只启动受 lifecycle context 管理的 Hub；handler 每连接只启动唯一 writePump；均有明确退出路径。
- reference import/dependency：无。

## Race

**Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT**。

未重复运行已知必然在 runtime/cgo 构建阶段失败的命令。共享连接状态由 Hub event loop 串行化，connection 写入由单 writePump 所有，并有并发逻辑发送/slow-client 测试。

**Race Verification Pending：必须在兼容的 64 位 C 工具链或 CI 环境执行 `go test -race ./...`。**

## 数据与后续范围

- Database migration：无变化；没有 `chat_messages` INSERT。
- Redis：无变化；未创建 Presence/online/connection tracking key。
- HTTP：新增内部 Chat API runtime 和 WebSocket Upgrade route；没有新增普通业务 API。
- WebSocket：只建立连接、typed boundary 和 outbound routing foundation；没有 Heartbeat/Ping/Pong、ACK、持久化、session/unread。
- Frontend：无变化。

## 修改文件

- `.env.example`、`.gitignore`
- `server/go.mod`、`server/go.sum`
- `server/cmd/chat-api/main.go`
- `server/internal/chat/**`
- `server/tests/chat_gateway_websocket_integration_test.go`
- `agent_logs/operations/day-2/002_checkpoint-1_chat-ws-foundation.md`
- `agent_logs/CURRENT_STATE.md`
- `agent_logs/AGENT_WORKLOG.md`
- `agent_logs/day-2.md`

## Git / 下一步

- Implementation commit：`b2e6f8cc2b10bc9cff834e0bf45e92cf7eacd932 feat(chat): add websocket hub and client foundation`。
- Log/status commit：本记录所在 `docs(day2): record chat websocket foundation` 提交。
- Push：两个提交完成后推送 `origin/main`，最终结果以 Git 复核为准。
- Day 2 blocker：NONE（race 是兼容环境验收缺口，不阻塞下一 Checkpoint）。
- Next Checkpoint：**Chat WebSocket Heartbeat + Connection Lifecycle**。
- 本操作到此停止，不进入下一 Checkpoint。
