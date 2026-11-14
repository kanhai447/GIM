# 002 / Task 1 — 好友上线提醒后端实现

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = eace570`。
- **结果**：后端实现 + 单测 + 真实 MySQL/Redis/WS 集成 **PASS**；未改 Hub 内部结构、Presence 写入、schema/Redis key。

## 修改文件

| 文件 | 变更 |
|---|---|
| `server/internal/chat/protocol/envelope.go` | `EventFriendOnline/Offline`、`FriendPresenceData{userID, at}` |
| `server/internal/chat/friendnotify/notifier.go` | 有界队列 observer + worker；好友查询、过滤、envelope、Hub.SendToUser、丢弃计数、drain shutdown；`GlobalPresenceCheck` 单机占位 |
| `server/internal/chat/friendnotify/notifier_test.go` | 入队/发送、自己过滤、查询失败、channel 满、drain shutdown、配置校验、并发 enqueue |
| `server/internal/chat/message/repository/mysql/friendship.go` | `FriendIDs`：`friends JOIN users`，`u.status=1`，跳过自己；已删除好友无行 |
| `server/internal/chat/config/config.go` | 可选 `FRIEND_NOTIFY_BUFFER`（默认 256）、`FRIEND_NOTIFY_WORKERS`（默认 1）、`FRIEND_NOTIFY_OPERATION_TIMEOUT`（默认 2s） |
| `server/cmd/chat-api/main.go` | `transitionObservers` 组合 Presence + friendnotify；SetSender、Start、Hub 关闭后 Shutdown drain |
| `server/tests/chat_message_integration_test.go` | `newChatMessageInstance` 支持传入 transition observer（variadic，旧调用不变） |
| `server/tests/friend_presence_integration_test.go` | 真实 WS 集成：online/offline、多端、非好友、禁用好友 |
| `.env.example`、`.env.compose.example` | 可选项注释（非必填） |

## 关键实现

- observer 回调只做非阻塞 `select` 入队；channel 满 `dropped++` 并日志，不阻塞 Hub。
- worker 用 background + 2s 超时，查询 `FriendIDs`，跳过 0/自己；对每个好友 `Hub.SendToUser`，失败只记录。
- `Shutdown`：写锁置 closed → close(events) → 等待 worker drain；enqueue 持读锁，避免 close 竞争。
- 单机多端由 Hub 的本地 0→1/1→0 保证；worker 不重复判断连接数。
- 多实例：`GlobalPresenceCheck` 接口占位，单机实现恒 true；V2 接 Redis 全局判断。
- 事件时间在 Hub 回调时生成；可能早于 Presence Redis 标记毫秒级，文档标注。

## 真实测试结果

```text
go build ./... / go vet ./... / gofmt -l .        PASS / PASS / 无输出
go test ./... -count=1                            PASS（70 packages）
go test ./internal/chat/friendnotify -count=1     PASS
GIM_ENV_FILE=<absolute> go test ./tests -run TestFriendPresenceNotifications -v   PASS
```

集成覆盖：

- A 登录 → B 收到 `friend.online`；
- A 多端登录 → B 只收到一次 online；
- 关闭 A 一端 → 无 offline；关闭最后一端 → B 收到 offline；
- 非好友 C 不收到；
- 禁用好友 D（status=2）不收到。

## 偏离/限制

- 单机语义；多实例重复/误报未解决（V2，V-20/K-15）。
- 事件 best-effort；进程崩溃/TTL 无 offline；channel 满丢弃（K-16）。
- 发送失败/查询失败不重试，记录脱敏日志（仅 ID/错误，无正文/JWT）。

## Git

- commit：`feat(chat): notify friends on presence transitions`（本记录所在提交）。
