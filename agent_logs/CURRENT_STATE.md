# GIM Current State

- **PROJECT:** GIM
- **V1 STATUS:** COMPLETED（文本 IM + 文件 + 好友闭环 + Admin + 审计）
- **V2 STATUS:** Phase 1 图片消息 COMPLETED（仅图片；视频/语音/普通文件/回复/引用/撤回/@/图文未实现）
- **Current Round:** 2026-11-12 Day 10 — V2 Phase 1 图片消息闭环：COMPLETED
- **Branch:** `main`
- **Tags:** `gim-v1`（2026-10-15 基线）、`gim-v1.1`（2026-11-12 验证状态版本）；均未移动，本轮未打新 tag
- **Next Authorized Work:** 图片代码 `-race`、群聊图片 E2E，或下一 V2 Phase（待用户决定）

## 当前事实

- 图片消息 `msg.type=2`：先 `POST /api/file/image` 上传，再经 `chat.send`/`group.send` 发送 `imageMsg.fileID`；服务端校验文件归属、`image/*` MIME 与 `IMAGE_MESSAGE_MAX_BYTES`（默认 10 MiB），用 DB 元数据规范化后与文本走同一事务、ACK、推送与历史。
- 错误码：Chat `2211–2214`（文件不存在/非本人、非图片、超限、参数非法），Group `2412–2415`；未知类型仍为 `2201/2403`；幂等比较含 `type+fileID`。
- 图片下载：`GET /api/file/:id` 允许所有者或引用该文件的聊天/群聊消息参与者；非参与者 `2503`；UUID 下载仍仅所有者。
- Web：私聊/群聊共用 `MessageBubble`，支持选择图片、上传、pending/ACK/重试、渲染、点击预览、失败占位；未知类型显示“暂不支持的消息”。
- 无 schema 变更：复用 `chat_messages.msg` / `group_messages.msg` 与 `user_files`/`file_objects`。
- 文档已同步：docs/06 §5.1–5.4、docs/05 §6/§7、docs/07 §2.1、docs/04 消息结构、README、validation-status、known-issues。

## 验证矩阵（本轮真实执行）

| 项目 | 结果 |
|---|---|
| `go build ./...` / `go vet ./...` / `gofmt -l .` | PASS / PASS / 无输出 |
| `go test ./... -count=1` | PASS（69 个包） |
| 聊天图片集成（真实 MySQL/Redis/WS） | **PASS**（ACK、接收、历史、重复 clientMsgId 仅 1 行、归属/MIME/大小、接收方下载授权） |
| 群聊图片 | PASS（单测；WS E2E 未单独跑） |
| Web `vue-tsc` / `vitest` / `vite build` | PASS / 21 files 114 tests / PASS |
| 浏览器图片 E2E（Chrome + Compose） | **PASS**（10 步：注册/好友/上传/发送/接收/预览/关闭） |

## 未验证（NOT RUN）

- 图片代码 `go test -race`（race 上一轮覆盖的是 V1 代码）。
- 群聊图片浏览器 E2E、图片吞吐/预览性能。
- 完整性能阶梯、soak/restart、文件吞吐、远端 CI（历史遗留）。

## Push

- 本轮 5 个功能/文档 commit 提交后推送到 `origin/main`；不使用 `--force`，不移动 tag。
- `gim-v1` / `gim-v1.1` 保持不变。
