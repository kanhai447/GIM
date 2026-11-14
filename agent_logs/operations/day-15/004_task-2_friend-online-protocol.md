# 004 / Task 2 — 好友上线提醒协议设计（不实现）

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = e3e37fd`。
- **结果**：协议写入 docs/06 §5.7，并同步 docs/05/07/08；**未写实现代码**。

## 阅读结论

| 现有结构 | 结论 |
|---|---|
| `chat.Hub` | 已有 `LocalConnectionTransitionObserver`：本地连接 0→1/1→0 回调，回调要求快速返回；Hub 事件循环不做外部 I/O |
| `presence.Service` | 实现该 observer，异步写 Redis ZSET `gim:presence:user:{uid}`（按实例 member + TTL） |
| `user/friend` | 好友列表通过 `Presence.IsOnline` 读 Redis；HTTP 只读 |
| `web/stores/friend.ts` | `friends[].isOnline` 由 HTTP 列表刷新；当前无 WS 事件 |
| 多实例 | Hub 本地计数不能代表全局在线；需结合 Redis Presence 判断全局 0→1/1→0 |

**无需改 Hub 并发模型**：observer 已存在且明确“不做 I/O”；实现只需新增/组合一个好友通知 worker（有界队列），通过现有 Chat delivery 推送。注意当前 `NewHub` 只接收一个 observer，实现时需组合 presence 与 notifier（或由 presence 服务统一分发），不改 Hub 内部结构。

## 协议要点

- 事件：`friend.online` / `friend.offline`，payload `{userID, at}`；仅 Chat WS。
- 触发：全局 Chat 连接 0→1 / 1→0；单机多端只首个/最后一个；多实例结合 Redis Presence。
- 范围：只通知该用户的好友（`friends` 表），不广播；跳过已删除好友/被封禁用户/自己；查询失败不推送。
- Presence：Redis 为权威；事件 best-effort；丢失后 HTTP `isOnline` 恢复；进程崩溃/TTL 过期不保证 offline 事件。
- 约束：Hub 回调只入队；复用 delivery/写队列；Group WS 不参与；旧客户端忽略未知事件。

## 修改文件

- `docs/06_WEBSOCKET_PROTOCOL.md`：§3 Presence 指向 §5.7；新增 §5.7；§9 事件表状态。
- `docs/05_HTTP_API.md`：好友 `isOnline` HTTP 读取与 WS 实时补充的关系。
- `docs/07_CORE_FLOWS.md`：登录流程标注；新增 §1.1 好友上线提醒流程。
- `docs/08_REDIS_CONFIG.md`：Presence key/TTL 不变，事件只读。
- `docs/known-issues.md`、`docs/validation-status.md`、`README.md`：设计完成、实现 NOT RUN。

## 未做

- 未实现 observer/worker/推送；未改前端；未新增表/列/索引；未打 tag。

## 验收

- 协议先行；未发现必须修改 Hub 并发模型或 Presence 写入方。

## Git

- commit：`docs: define friend online reminder protocol`（本记录所在提交）。
