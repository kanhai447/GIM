# Day 2 / Operation 003 — Chat WebSocket Heartbeat + Connection Lifecycle

- **时间：** 2026-10-10 10:01 +08:00
- **状态：** COMPLETED
- **范围：** 仅补齐 Chat WebSocket Ping/Pong、deadline、异常连接清理、连接统计、生命周期日志和测试；未实现 Presence、Redis online、ACK、消息持久化、session/unread 或前端 socket。

## 开始状态

- Branch：`main`。
- 开始 HEAD：`857d264827071b437ae22f0c45eb1da38c472378`。
- `git fetch origin`：SUCCESS。
- 开始时 `HEAD == origin/main`：PASS。
- Working tree：CLEAN。
- Chat WebSocket Foundation：COMPLETED。

## Heartbeat

### Ping/Pong model

```text
connect/register
  -> readPump sets initial read deadline and control handlers
  -> writePump ticker emits WebSocket Ping
  -> peer Pong is consumed by readPump
  -> PongHandler only refreshes read deadline
  -> missing Pong reaches read deadline
  -> readPump exits and asks Hub to unregister this Client
```

- Heartbeat 使用 WebSocket control frame，不新增 JSON 业务 ping，也没有改变 typed envelope。
- writePump 是生产应用代码唯一调用 `Conn.WriteMessage` / `Conn.WriteControl` 的位置。
- 为避免 Gorilla 默认 PingHandler 从 readPump 回写 Pong，GIM 将 peer Ping 响应放入容量 4 的内部 control queue，再由 writePump 写出。
- CloseHandler 不从 readPump 写回控制帧；ReadMessage 返回 CloseError 后按正常/异常原因清理。
- Gorilla 对非法底层 frame 的内部协议处理属于库内部行为；GIM 业务/handler/Hub/readPump 没有绕过 writePump 的显式连接写入。

### Production defaults

- `CHAT_WS_READ_LIMIT_BYTES=1048576`（1 MiB，允许范围 1–16 MiB）。
- `CHAT_WS_PONG_WAIT=60s`。
- `CHAT_WS_PING_PERIOD=50s`，强制 `< pongWait`。
- `CHAT_WS_WRITE_WAIT=10s`。
- heartbeat duration 必须为正且不超过 10m；测试通过显式注入短 duration，不把毫秒值写入生产模板。

### Deadline model

- readPump 在开始读取前设置 read limit 和 `now + pongWait` read deadline。
- 每个合法 Pong 仅执行 `SetReadDeadline(now + pongWait)`。
- writePump 在每个 data/control frame 前设置 write deadline；Ping 的 control deadline 同样为 `tick + writeWait`。
- write deadline/写失败、read timeout、非法/异常关闭均终止对应 pump 并触发幂等注销。

## Lifecycle / ownership

- Hub event loop 继续唯一拥有 `userID -> clientID -> Client` map。
- Hub 接受 register/unregister/count/route 命令；新增 `TotalConnectionCount`，不产生 Presence 副作用。
- Hub 删除 Client 后调用唯一 `Client.stop(reason)`；`sync.Once` 保证 connection 与 Send 只关闭一次。
- **Send channel 的唯一关闭点是 `Client.stop`，且只有 Hub 移除/Hub shutdown 路径触发。** readPump、writePump 不直接关闭 Send。
- control queue 不关闭：它有界且只在 Client 生命周期内使用；这样读侧 control handler 与 shutdown 并发时不会发生 send-on-closed-channel。Conn/Send 关闭使 pumps 退出，随后 channel 随 Client 回收。
- readPump、writePump、slow-client removal、shutdown 都可能请求注销；Hub 的 identity check + `stopOnce` 使重复 unregister/close 安全。
- writePump 创建并独占 heartbeat ticker，在所有返回路径 `defer ticker.Stop()`；测试显式等待 `writeDone` 并确认退出后不再产生 Ping。
- lifecycle reason 区分 `normal`、`timeout`、`abnormal`、`slow_client`、`shutdown`；日志只包含 userID、opaque ClientID 和 reason，不记录 Token/JWT/query/Authorization/Secret。

## Disconnect and shutdown behavior

- 正常 CloseNormalClosure / GoingAway：readPump 分类 normal，移除 Client，不重复刷异常日志。
- read deadline：分类 timeout，移除且关闭 Conn/Send，read/write pumps 退出。
- raw TCP abrupt close、非法/其他 read failure：分类 abnormal，清理且不 panic。
- slow client：有界 Send 满时 Hub 非阻塞移除单个 Client；Conn close 解除可能阻塞的 write，ticker/writePump 退出。
- server + Hub shutdown：HTTP listener 先停止接受新连接，Hub 在 timeout 内关闭全部 clients，read/write pumps 和 ticker 退出；shutdown 后新 WebSocket dial 失败。

## Multi-device lifecycle

- 同一用户两个连接测试：A 不读取 Ping，因此不发送 Pong；B 持续读取并明确响应 Pong。
- A 在 read deadline 后以 timeout 被移除；B 继续跨越另一个 pongWait 存活。
- user connection count：`2 -> 1 -> 0`；初始 total connection count 为 2。
- 一个设备 timeout 不会删除 sibling Client，没有 zombie Client。
- 本轮 connection count 不触发 Redis、online/offline event 或好友通知。

## FIM comparison

- FIM Chat/Group WebSocket 没有 Ping/Pong、read deadline、Pong deadline refresh 或 dead connection detection；断线清理由 read failure 被动驱动，且普通全局 map/多路径直写 Conn 使生命周期不清晰。
- GIM 自主实现配置化 heartbeat、Hub 单所有者、单 writePump、精确 Client 注销、slow/dead/shutdown 原因分类和可等待的 pump 退出测试。
- 未复制 FIM WebSocket 文件、heartbeat 代码或连接 map；reference 仍为只读且不是依赖。

## Gateway tunnel regression

真实临时单节点 etcd + Auth handler + Gateway discovery/reverse proxy + go-zero Chat REST route：

- valid JWT WebSocket connect：PASS。
- Ping 穿过 Gateway：PASS。
- 测试客户端连续 Pong，连接保持超过 pongWait：PASS。
- connection close 后 Hub count 回到 0：PASS。
- missing/logout JWT 拒绝回归：PASS。
- 临时 etcd 测试完成后已关闭，`etcd_process=NONE`。

## Tests

- `go test ./... -count=1`：PASS。
- `go vet ./...`：PASS。
- `gofmt -l .`：PASS（无输出）。
- `git diff --check`：PASS。
- `go test ./internal/chat/... -coverprofile=... -count=1`：PASS。
- Chat heartbeat/lifecycle 汇总 coverage：**81.5% statements**；`internal/chat` 78.8%，config 96.9%，protocol 100%。
- heartbeat/lifecycle 9 个核心测试组合 `-count=10`：PASS。
- Pong refresh、server Ping、peer Ping 经 writePump 回 Pong、write deadline、ticker stop：PASS。
- missing Pong timeout、normal close、abrupt close、double cleanup、slow client、Hub/server shutdown：PASS。
- 同用户 timeout-one/keep-sibling：PASS。
- Gateway heartbeat tunnel regression（真实 etcd）：PASS，1.17s。
- Origin allow/reject 回归：PASS。
- lifecycle safe logging：PASS。
- `scripts/check-repository-safety.ps1`：PASS。

## Code audit

- `time.NewTicker`：生产代码仅 writePump 一处，且同一函数 defer Stop。
- `WriteMessage`：生产实现仅 socket interface 声明 + writePump data write。
- `WriteControl`：生产实现仅 socket interface 声明 + writePump Ping/control queue write。
- `WriteJSON` / `NextWriter`：生产 Chat 代码无命中。
- `SetReadDeadline` / `SetPongHandler`：只在 readPump 初始化/刷新路径。
- `SetPingHandler`：只向有界 control queue 入队，不直接写 Conn。
- `close(client.send)`：唯一位于 `Client.stop`，由 Hub 生命周期调用并受 `stopOnce` 保护。
- `go func`：Hub、每连接 writePump、runtime server/test reader 均有 context/channel/Conn 关闭路径；测试显式等待关键 goroutine 完成。
- Presence/Redis key、DB insert、ACK/clientMsgId 业务处理：无新增。

## Race

**Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT**。

环境状态未变化，未重复运行已知会在 runtime/cgo 阶段以 `64-bit mode not compiled in` 失败的完整 race。Hub 状态由 event loop 串行化；writePump 独占应用层连接写；stopOnce 和有界 channel 控制重复清理/背压。

**Race Verification Pending：必须在兼容的 64 位 C 工具链或 CI 环境执行 `go test -race ./...`。**

## 数据/协议/安全变化

- Database：无变化；没有消息写入。
- Redis：无变化；未创建 Presence/online key。
- HTTP/JWT：无变化；仍由 Gateway/Auth 验证 query token。
- WebSocket application envelope：无变化；仅增加标准 control-frame heartbeat。
- Frontend：无变化。
- Secret Scan：PASS；未记录/提交 JWT、Token、Authorization、Secret 或本地凭据。

## 修改文件

- `.env.example`
- `server/internal/chat/client.go`
- `server/internal/chat/lifecycle.go`
- `server/internal/chat/hub.go`
- `server/internal/chat/handler.go`
- `server/internal/chat/module.go`
- `server/internal/chat/config/config.go`
- `server/internal/chat/**/*_test.go`
- `server/tests/chat_gateway_websocket_integration_test.go`
- `agent_logs/operations/day-2/003_checkpoint-2_chat-heartbeat-lifecycle.md`
- `agent_logs/CURRENT_STATE.md`
- `agent_logs/day-2.md`
- `agent_logs/AGENT_WORKLOG.md`

## Git / 下一步

- Implementation commit：`2b90df522337932bb57dfb0889caf39f54da80d3 feat(chat): add websocket heartbeat and connection lifecycle`。
- Log/status commit：本记录所在 `docs(day2): record heartbeat lifecycle checkpoint` 提交。
- Push：两个提交完成后推送 `origin/main`，最终结果以 Git 复核为准。
- Day 2 blocker：NONE（race 是兼容环境验收缺口，不阻塞下一 Checkpoint）。
- Next Checkpoint：**Chat Presence**。
- 本操作到此停止，不进入 Presence。
