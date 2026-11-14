# GIM Current State

- **PROJECT:** GIM
- **V1 STATUS:** COMPLETED（文本 IM + 文件 + 好友闭环 + Admin + 审计）
- **V2 STATUS:** Phase 1 完成（图片 `type=2` + 普通文件 `type=3`）；撤回协议已定义、实现待第十一轮；语音/视频/回复/引用/@/图文未实现
- **Current Round:** 2026-11-14 Day 13 — 文件 Phase 收尾（race + 群聊文件 E2E）+ 撤回协议设计：COMPLETED
- **Branch:** `main`
- **Tags:** `gim-v1`（09994cb）、`gim-v1.1`（55bbb8e）、`gim-v1.2`（c97618f）；均未移动，本轮未打新 tag
- **Next Authorized Work:** 撤回实现（第十一轮）/ 好友上线提醒 / user_confs / 图片与文件性能

## 当前事实

- 图片消息 `type=2`、普通文件消息 `type=3` 已实现并验证；文件消息不限制 MIME 前缀，`FILE_MESSAGE_MAX_BYTES` 默认 25 MiB。
- 撤回协议已定义（docs/06 §5.6）：`msg.type=4` + `withdrawMsg.msgID`，沿用 `chat.send`/`group.send`；校验存在/本人/未 tombstone/未撤回/`RECALL_WINDOW_SECONDS`（默认 120）/群成员；Chat `2218–2221`、Group `2419–2422`；历史替换为撤回占位、`[撤回]`；文件授权对非所有者失效。
- **撤回尚未实现**：服务端收到 `type=4` 仍返回 `2201/2403`；前端无撤回入口；无新表/列/索引。
- 下载授权：所有者/私聊参与者/群成员（消息未 tombstone）；退群/非参与者 `2503`；授权 SQL 同时匹配 `imageMsg.fileID`/`fileMsg.fileID`。
- 文档与 `.env.example` 已预留 `RECALL_WINDOW_SECONDS=120`（第十一轮实现使用）。

## 验证矩阵（真实执行）

| 项目 | 结果 |
|---|---|
| 全量 race（Linux Docker） | **PASS**（exit 0，无 race warning，覆盖 chat/group/file/friend） |
| 群聊文件浏览器 E2E | **PASS**（B/C 下载、D `2503`、B 退群 `2503`、owner 200） |
| 聊天文件集成（前一轮） | PASS |
| 文件下载授权专项测试（前一轮） | PASS |
| Web | PASS（21 files / 122 tests；本轮无前端改动） |
| 撤回协议设计 | **完成**；实现 **NOT RUN** |

## 未验证（NOT RUN）

- 撤回后端/前端实现与 E2E（第十一轮）。
- 图片/普通文件消息吞吐与预览性能。
- 完整性能阶梯、soak/restart、文件吞吐、远端 CI（历史遗留）。

## Tag / Push

- `gim-v1`/`gim-v1.1`/`gim-v1.2` 均未移动；本轮未打新 tag。
- 本轮协议/文档 commit 推送至 `origin/main`；不 `--force`。
