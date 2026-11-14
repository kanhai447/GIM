# GIM Current State

- **PROJECT:** GIM
- **V1 STATUS:** COMPLETED（文本 IM + 文件 + 好友闭环 + Admin + 审计）
- **V2 STATUS:** Phase 1 完成（图片 `type=2` + 普通文件 `type=3`）；语音/视频/回复/引用/撤回/@/图文未实现
- **Current Round:** 2026-11-14 Day 12 — gim-v1.2 tag + 普通文件消息 Phase 1：COMPLETED
- **Branch:** `main`
- **Tags:** `gim-v1`（09994cb）、`gim-v1.1`（55bbb8e）、`gim-v1.2`（c97618f）；均未移动，本轮未打 `gim-v2.0`
- **Next Authorized Work:** 撤回 / 好友上线提醒 / `user_confs` / 图片与文件消息性能（待用户决定）

## 当前事实

- 普通文件消息 `msg.type=3`：先 `POST /api/file/file` 上传，再发送 `fileMsg.fileID`；服务端校验归属与 `FILE_MESSAGE_MAX_BYTES`（默认 25 MiB），**不限制 MIME 前缀**，回填 `url/fileName/size/mimeType`；`msg_preview="[文件]"`。
- 错误码：Chat `2215` 文件不存在/非本人、`2216` 超限、`2217` 参数；Group `2416–2418`；未知类型 `2201/2403`；幂等冲突 `2205/2407`。
- 下载授权：图片与文件消息统一——所有者始终允许；私聊发送者/接收者且消息未 tombstone；当前群成员且消息未 tombstone；退群/非参与者 `2503`。授权 SQL 同时匹配 `imageMsg.fileID` 与 `fileMsg.fileID`，`SELECT 1 … LIMIT 1`。
- Web：私聊/群聊共用 `MessageBubble`；文件卡片显示文件名/大小，下载走带 token 的 File API（Blob + `<a download>`），失败可重试。
- 无 schema/索引/migration 变更；复用 `msg` JSON 与 `user_files`/`file_objects`。

## 验证矩阵（真实执行）

| 项目 | 结果 |
|---|---|
| 后端 `go build` / `go vet` / `gofmt` / `go test ./...` | PASS / PASS / 无输出 / 69 包 PASS |
| 聊天文件集成（真实 MySQL/Redis/WS） | **PASS**（ACK、接收、历史、重复 clientMsgId 仅 1 行、`2215/2216` 不落库、接收方可下载） |
| 群聊文件消息 | PASS（单测；授权测试含群文件消息） |
| 文件下载授权 | **PASS**（所有者/私聊/群成员/tombstone/退群/非参与者/未知 fileID/文本不授权） |
| Web `vue-tsc` / `vitest` / `vite build` | PASS / 21 files 122 tests / PASS |
| 浏览器文件 E2E | **PASS**（A 发 `report.pdf` → B 下载 → D `2503`） |
| 图片代码 race（前一轮） | PASS（本轮未新增并发代码，未复跑） |

## 未验证（NOT RUN）

- 图片与普通文件消息的上传/下载吞吐、预览性能。
- 完整性能阶梯、soak/restart、文件吞吐、远端 CI（历史遗留）。

## Tag / Push

- `gim-v1.2` 已于本轮创建并推送；`gim-v1`、`gim-v1.1` 未移动。
- 本轮代码/文档 commit 推送至 `origin/main`；不 `--force`。
