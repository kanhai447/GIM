# 001 / Task 1 — 好友上线提醒后端设计（编码前）

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = eace570`，工作区干净。
- **授权**：docs/06 §5.7；单机语义；不改 Hub 内部结构、不改 Presence 写入、不新增 schema/Redis key。

## 现状阅读结论

| 位置 | 结论 |
|---|---|
| `chat.Hub` | `registerClient` 在本地连接数 0→1 调 `LocalUserOnline`；`removeClient` 在 1→0 调 `LocalUserOffline`；回调在 Hub 事件循环内同步执行，必须快速返回 |
| `NewHub` | 只取 `observers[0]`；不修改 Hub，在 chat-api 装配层组合两个 observer |
| `presence.Service` | 已实现同一接口；Presence 写入逻辑不变 |
| `chat` FriendshipRepository | 已只读 `friends` 表（`AreFriends`）；新增 `FriendIDs` 属于同一 Chat 侧只读仓储，不新增 RPC/服务边界 |
| `Hub.SendToUser` | 经 Hub command + 每 client `send` 的非阻塞 select；worker 调用不会直接写 Conn |
| 好友过滤 | `friends` 表删除是双向物理删除；`users.status=1` 表示启用；SQL 中过滤 `u.status=1` 与 `friend_id<>userID` |
| 多实例 | 本轮只做单机；worker 内 `GlobalPresenceCheck` 接口占位，单机返回 true，V2 接 Redis 全局判断 |

## 设计

新增 `server/internal/chat/friendnotify/`：

- `Notifier` 实现 `chat.LocalConnectionTransitionObserver`；
- 回调只做非阻塞入队：`select { case events <- e: default: dropped++/log }`；
- 有界 buffered channel；容量/worker 数从 chat config 读取，默认 256 / 1；不新增必填 env；
- worker 消费：
  1. `GlobalPresenceCheck.ShouldNotify`（单机 true）；
  2. `FriendLookup.FriendIDs(ctx, userID)`；
  3. 跳过 0/自己；
  4. 构造 `friend.online`/`friend.offline` envelope（`{userID, at}`）；
  5. 对每个好友 `Sender.SendToUser`，单次有界超时，失败只记录；
- 发送失败/查询失败不重试、不阻塞；channel 满记录丢弃计数；
- `Shutdown(ctx)`：加写锁置 closed → close(events) → 等待 worker drain；enqueue 持读锁做非阻塞发送，避免 close 竞争。

装配（`cmd/chat-api/main.go`）：

```go
notifier := friendnotify.New(friendRepo, cfg, logger)
composite := transitionObservers{presenceService, notifier}
module := chat.NewWithInboundFactory(values, factory, composite)
notifier.SetSender(module.Hub)
notifier.Start(lifecycleContext)
// shutdown: Hub.Shutdown -> notifier.Shutdown -> presence shutdown
```

顺序保证 Hub 关闭时产生的 offline 事件仍能入队，随后 worker drain。

## 语义

- 单机多端：observer 只在 0→1/1→0 回调，worker 不再判断连接数。
- 好友过滤：`FriendIDs` 只返回 `u.status=1` 且非自己的好友；已删除好友无行；被封禁/停用被 SQL 排除；查询失败不发送。
- 事件是 best-effort；接收方离线时 `SendToUser` 返回 0 投递，不重试。
- Group WS 不参与。

## 测试计划

- 单测：入队、好友过滤（自己）、查询失败、channel 满丢弃、shutdown drain、envelope/payload。
- 集成（真实 MySQL/Redis/WS）：A 登录→B 收 online；A 多端→只一次 online；关一端→无 offline；关最后→offline；非好友 C 不收到。
- race：全量 Linux 容器。

## 预期偏离

- 协议 §5.7 写“Presence 标记后”，现有 Presence 写入是异步 worker；好友事件在 Hub 回调时入队，可能早于 Redis 标记毫秒级。事件只作通知，HTTP `isOnline` 仍是权威；在文档标注。
- 多实例全局 0→1/1→0 不实现（授权要求），留接口与 V2 标注。
