# Day 2 / Operation 004 — Chat Presence

- **时间：** 2026-10-10 10:21 +08:00
- **状态：** COMPLETED
- **范围：** 只实现 Chat WebSocket Presence；未实现好友广播、Group Presence、消息持久化、ACK、clientMsgId 业务、session/unread、read receipt 或前端 socket。

## 开始状态

- Branch：`main`。
- 开始 HEAD：`b2715c2b18999ffb448045e66e20b9e46c8a486a`。
- `git fetch origin`：SUCCESS。
- 开始时 `HEAD == origin/main`：PASS。
- Working tree：CLEAN。
- Checkpoint 1 Hub/Client 与 Checkpoint 2 heartbeat/lifecycle：COMPLETED。

## Presence model

```text
Chat Hub local connection transitions
        -> non-blocking desired state
Presence Service worker
        -> context timeout / retry / refresh
Redis ZSET per user
```

- 本实例连接事实仍由 Hub event loop 独占的 `userID -> clientID -> Client` map 维护；Redis 不反向承担单机连接计数。
- Hub 只调用快速的 `LocalConnectionTransitionObserver`，不在单 event loop 内执行 Redis 网络 I/O。
- Presence Service 使用内存 desired/dirty 状态与容量 1 的 coalescing wake；Redis 失败保留最终期望状态，按重试周期恢复，不为每个失败创建 goroutine。
- `IsOnline(ctx, userID)` 位于独立 Presence 边界，不绑定 HTTP Handler；本轮没有接入好友列表展示。
- DB `online` 字段不是 Presence truth，本轮没有写 MySQL online 状态。

## Redis structure / instance identity / TTL

- 集中 helper：`rediskeys.PresenceUser(uid)` -> `gim:presence:user:{uid}`；业务代码无分散 key 拼接，key 不含 JWT、ClientID 或设备信息。
- Redis value 为 ZSET：member=`CHAT_API_INSTANCE_ID`，score=`expiresAtUnixMillis`。
- 全局 ONLINE：至少一个未过期 Chat instance contribution；全局 OFFLINE：清理过期 member 后 ZCARD 为 0。
- `MarkOnline` 原子 pipeline 写 ZSET member 并设置 key PTTL；`MarkOffline` Lua 删除本实例 member，空集合时删除 key；`IsOnline` Lua 原子清理过期 member 并计数。
- Instance ID 复用现有 etcd/service registration 的 `CHAT_API_INSTANCE_ID`，不从 JWT/用户/heartbeat 生成。
- 生产默认 TTL 90s、refresh 30s、retry 5s、operation timeout 1s；独立 worker 刷新，不把每个 WebSocket Ping 变成 Redis 写入。
- crash 或 Redis offline cleanup 失败时，member score + key PTTL 最终清除 stale Presence；正常 shutdown 则在 Hub 清空后显式移除本实例 contributions。

## Transitions / multi-device

- 第一个 Client register 成功并进入 Hub：`0->1`，恰好一次 local online contribution transition。
- 同用户继续 register：`1->N`，不重复 online transition。
- 任一 normal/timeout/slow Client 离开但仍有 sibling：`N->1`，不触发 offline。
- 最后一个 Client 移除：`1->0`，恰好一次 local offline contribution transition。
- duplicate unregister 不会重复 offline；register 未成功不会产生 ghost online。
- heartbeat timeout、slow-client removal、normal close 与 shutdown 没有第二套 Presence listener，全部统一通过 Hub connection count transition。

## Multi-instance

- Instance A 与 B 可同时写同一用户的不同 ZSET member。
- A disconnect/remove contribution 后 B 仍有效：全局 ONLINE。
- A crash 不清理时，其 member 到期；B 独立 refresh 后用户仍 ONLINE。
- 最后 B disconnect 后无有效 contribution：全局 OFFLINE。
- Presence contribution 已具备多 Chat API 实例安全语义；跨实例消息路由仍是 V2，本轮未实现或虚构 Pub/Sub/Stream。

## Redis failure / shutdown

- 所有 store/query 操作使用 caller context + `CHAT_PRESENCE_OPERATION_TIMEOUT`。
- timeout/unavailable 时 Hub register、count、unregister 均不等待 Redis；worker 记录安全错误并保留 dirty state。
- Redis recovery 后 retry 将当前最终 desired state 写回；在线用户还按 refresh 周期续租。
- shutdown 请求使用有界高优先通道；当前 Redis 操作超时后优先执行最终 offline flush，避免被 refresh/retry wake 饿死。
- cleanup 失败最多等待 operation/shutdown context，随后由 TTL 兜底，不永久卡住进程。

## Lifecycle results

- normal close：最后连接 offline，PASS。
- heartbeat timeout：一个设备 timeout、健康 sibling 保持 online；最后 timeout 只产生一次 offline，PASS。
- slow client：slow sibling 移除不使用户 offline；最后/其他 active user 在 shutdown 各产生一次 offline，PASS。
- shutdown：Hub 先停止/清空 Client，再由 Presence Service flush 本实例 contributions；无 hanging worker，PASS。

## Gateway / real Redis integration

- 真实本地 Redis：双实例 online、A offline/B remains、A crash TTL/B refresh、last B offline、临时 key 清理，PASS。
- 真实临时 etcd + Auth + Gateway discovery/reverse proxy + Chat + Redis：有效 query JWT connect 后 ONLINE；同用户双连接关闭一个仍 ONLINE；关闭最后连接 OFFLINE；dead sibling heartbeat timeout 后仍 ONLINE；最后健康连接关闭 OFFLINE，PASS。
- 首次 Gateway 回归因本地 etcd 未运行而 health check 失败；启动官方临时 etcd 后通过，测试后进程、data 与 binary 已清理。
- 后续仅为重复最终 Gateway 回归下载临时 etcd 时出现外部 TLS EOF；没有改变上述已通过的真实 tunnel 结果，最终当前代码由全量/聚焦测试继续覆盖编译和生命周期行为。

## FIM comparison

- 只读参考 FIM Chat endpoint 的 `UserOnlineWsMap`、多连接 map、Redis `online` Hash 写删和好友上线行为；未复制文件或业务实现。
- FIM 使用 package 全局普通 map、每次连接均 HSet、无 TTL/实例 contribution，且连接/通知路径存在并发直写 Conn；多设备和多实例在线语义不安全。
- GIM 自主实现 Hub 单所有者、用户级 transition、独立异步 Presence worker、多实例 ZSET contribution、TTL crash recovery、context timeout/retry 和单 writePump 既有边界。

## Authority

- **Chat WebSocket = global Presence 唯一权威来源。**
- **Group WebSocket 不得 MarkOnline、MarkOffline 或修改 `gim:presence:*`。**
- 本轮没有实现 Group WS、好友查询/广播、WS presence 消息、Kafka 或前端 Presence UI。

## Tests

- `go test ./... -count=1`：PASS。
- `go vet ./...`：PASS。
- `gofmt -l .`：PASS（无输出）。
- `git diff --check`：PASS。
- `go test ./internal/chat/presence -cover -count=1`：PASS，**75.8% statements**。
- 5 个关键 transition/timeout/slow/shutdown/Redis-blocking 测试组合 `-count=10`：PASS。
- Redis normal、operation timeout、unavailable、recovery、refresh past TTL、shutdown cleanup：PASS。
- real Redis multi-instance/TTL integration：PASS；测试 key 最终不存在。
- Gateway/Auth/Chat/Redis WebSocket integration：PASS。
- `scripts/check-repository-safety.ps1`：PASS。

## Race

**Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT。**

按要求未重复运行已知必然在 Windows runtime/cgo 阶段失败的完整 race；状态保持：

**Race Verification Pending：必须在兼容 64 位 C 工具链或 CI 环境执行 `go test -race ./...`。**

## 数据/协议变化

- Database：无变化；没有写 users.online、chat_messages、session 或 unread。
- HTTP：无新增业务 API。
- WebSocket application envelope：无变化；没有新增 Presence event/好友广播。
- Redis：新增集中 Presence key、ZSET instance contribution、TTL/refresh/retry 配置。
- Frontend：无变化。

## 修改文件

- `.env.example`
- `docs/02_BACKEND_DESIGN.md`
- `docs/07_CORE_FLOWS.md`
- `docs/08_REDIS_CONFIG.md`
- `server/cmd/chat-api/main.go`
- `server/internal/chat/hub.go`、`module.go` 与相关 lifecycle/integration tests
- `server/internal/chat/presence/**`
- `server/internal/platform/rediskeys/**`
- `server/tests/chat_gateway_websocket_integration_test.go`
- `server/tests/chat_presence_redis_integration_test.go`
- `agent_logs/operations/day-2/004_checkpoint-3_chat-presence.md`
- `agent_logs/CURRENT_STATE.md`
- `agent_logs/day-2.md`
- `agent_logs/AGENT_WORKLOG.md`

## Git / 下一步

- Implementation commit：`6bd5c4506bdccf237c3652d0af1e06587a691c41 feat(chat): add redis backed user presence`。
- Log/status commit：本记录所在 `docs(day2): record chat presence checkpoint` 提交。
- Push：两个提交完成后推送 `origin/main`，最终结果以 Git 复核为准。
- Day 2 blocker：NONE（race 是兼容环境验收缺口，不阻塞下一 Checkpoint）。
- Next Checkpoint：**Private Message Persistence + clientMsgId + ACK**。
- 本操作到此停止，不进入消息持久化、ACK、session/unread 或前端 socket。
