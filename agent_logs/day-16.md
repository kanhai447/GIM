# Day 16 — 好友上线提醒实现 + gim-v1.4（2026-11-14）

## 今日目标

按 docs/06 §5.7 实现好友上线提醒（后端通知、前端实时更新、测试/文档），打 `gim-v1.4`；单机语义，不新增 schema/Redis key，不改 Hub/Presence 写入。

## 真实结果

| 项目 | 结果 | 证据 |
|---|---|---|
| 后端单测 | **PASS** | `go test ./...` 70 packages；friendnotify 7 类单测 |
| 好友提醒集成（真实 MySQL/Redis/WS） | **PASS** | online/offline、多端只一次、关一端无 offline、非好友/禁用好友不通知 |
| Web | **PASS** | `vue-tsc`；`vitest` 21 files / **135 tests** |
| 浏览器 E2E | **PASS** | B 看到 A 在线 → A 关闭离线 → A 重登在线 |
| 全量 race | **PASS** | Linux `go test -race ./...`，exit 0，无 DATA RACE |
| 多实例全局 Presence | **NOT RUN** | 本轮只做单机；`GlobalPresenceCheck` 占位（V2） |

## 关键实现

- `chat/friendnotify`：observer 非阻塞入队（默认 256/1 worker/2s，可配）；worker 查询 `friends JOIN users status=1`，跳过自己/已删除/禁用；`Hub.SendToUser` 投递；满队列丢弃计数；shutdown drain。
- chat-api 装配层组合 Presence + friendnotify，Hub 关闭后 drain notifier。
- 前端 Chat WS 解析 `friend.online/offline`，friend store 按 userID 更新已知好友；未知忽略；HTTP 恢复。
- 未改 Hub 内部、Presence 写入、Group WS、schema。

## Commit / Tag

- `db625c5` feat(chat): notify friends on presence transitions
- `6f8ea2d` feat(web): apply friend online events
- `886d55e` docs: record friend online reminder support
- annotated `gim-v1.4`：tag object `2cf99b5`，target `886d55e`
- main push `eace570..886d55e`；tag push new `gim-v1.4`

## 未验证 / 剩余

- 多实例全局 0→1/1→0（V2/V-20）；事件 best-effort，崩溃/TTL 无 offline（K-16）。
- 好友申请实时推送、user_confs、管理员撤回、性能/soak/远端 CI 仍未做。
- K-14 并发撤回严格单条仍需 schema 决策。

## 下一步

1. 好友上线提醒多实例（或好友申请实时推送）。
2. `user_confs` / 管理员撤回。
3. 性能：图片/文件吞吐、完整阶梯。
4. 是否准备 `gim-v2.0`（另行授权）。
