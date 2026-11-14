# GIM Current State

- **PROJECT:** GIM
- **V1 STATUS:** COMPLETED（文本 IM + 文件 + 好友闭环 + Admin + 审计）
- **V2 STATUS:** Phase 1 完成（图片 `type=2`、普通文件 `type=3`、撤回 `type=4`、好友上线提醒单机）；多实例 Presence/user_confs/管理员撤回未实现
- **Current Round:** 2026-11-14 Day 16 — 好友上线提醒实现 + `gim-v1.4`：COMPLETED
- **Branch:** `main`
- **Tags:** `gim-v1`（09994cb）、`gim-v1.1`（55bbb8e）、`gim-v1.2`（c97618f）、`gim-v1.3`（32a4a55）、`gim-v1.4`（886d55e）；旧 tag 未移动
- **Next Authorized Work:** 好友上线提醒多实例 / 好友申请实时推送 / `user_confs` / 性能 / `gim-v2.0`（需授权）

## 当前事实

- 好友上线提醒已实现（单机）：Chat Hub 本地 0→1/1→0 → `friendnotify` 有界 worker → `friends JOIN users(status=1)` → `Hub.SendToUser` 推送 `friend.online`/`friend.offline`；前端 Chat WS 更新 `isOnline`；HTTP 好友列表仍为权威恢复路径。
- 队列默认 256 / 1 worker / 2s 超时，可配；满队列丢弃并计数；shutdown drain。
- 过滤：自己、已删除好友、禁用用户；非好友不通知；Group WS 不参与。
- 多实例全局 Presence 判断未实现（`GlobalPresenceCheck` 占位，V2）；崩溃/TTL 不保证 offline（best-effort）。
- 撤回 `type=4`、图片/文件、好友闭环保持既有实现；无 schema/索引变更。

## 验证矩阵（真实执行）

| 项目 | 结果 |
|---|---|
| 后端 `go build` / `go vet` / `gofmt` / `go test ./...` | PASS / PASS / 无输出 / 70 包 PASS |
| 好友提醒集成（真实 MySQL/Redis/WS） | **PASS**（online/offline、多端、非好友、禁用好友） |
| Web `vue-tsc` / `vitest` | PASS / 21 files 135 tests |
| 浏览器 E2E | **PASS**（在线→离线→重登在线） |
| 全量 race | **PASS**（exit 0，无 DATA RACE） |

## 未验证（NOT RUN）

- 多实例好友上线提醒（V-20/K-15）。
- 好友申请实时推送、`user_confs`、管理员撤回。
- 图片/文件吞吐、完整性能阶梯、soak/restart、远端 CI。
- K-14 并发撤回严格单条（需 schema 决策）。

## Tag / Push

- `gim-v1.4`：tag object `2cf99b5`，target `886d55e`；已推送。
- 旧 tag peeled commit 未变：09994cb / 55bbb8e / c97618f / 32a4a55。
- main `eace570..886d55e` 已推送；不 `--force`。
