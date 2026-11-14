# GIM Current State

- **PROJECT:** GIM
- **V1 STATUS:** COMPLETED（文本 IM + 文件 + 好友闭环 + Admin + 审计）
- **V2 STATUS:** Phase 1 图片消息 COMPLETED（含 race/授权/群聊 E2E 收尾）；其余富消息未实现
- **Current Round:** 2026-11-14 Day 11 — 图片 Phase 1 收尾（授权审计/测试、race、群聊图片 E2E）：COMPLETED
- **Branch:** `main`
- **Tags:** `gim-v1`、`gim-v1.1`；均未移动，本轮未打新 tag
- **Next Authorized Work:** 图片吞吐/预览性能，或下一 V2 Phase（待用户决定）

## 当前事实

- 图片消息 `msg.type=2`：上传 `POST /api/file/image` → `chat.send`/`group.send` 发送 `imageMsg.fileID`；服务端校验归属/`image/*`/`IMAGE_MESSAGE_MAX_BYTES`，事务落库、ACK、推送、历史同构。
- 文件下载授权（`GET /api/file/:id`）：
  - 所有者始终允许；
  - 私聊发送者/接收方，且引用消息未 tombstone；
  - 当前群成员，且群消息未 tombstone；
  - 退群/被踢、tombstone 后、非参与者统一 `2503`；
  - 文本消息不构成文件引用、不触发下载。
- 授权查询只读现有表，`SELECT 1 … LIMIT 1`，无 schema/索引变更；JSON 无索引与无速率限制已登记。
- 浏览器已缓存的图片副本不因退群/tombstone 回收（客户端缓存语义）。
- Web：私聊/群聊共用 `MessageBubble`，支持上传、pending/ACK/重试、渲染、预览、失败占位。

## 验证矩阵（真实执行）

| 项目 | 结果 |
|---|---|
| 后端 `go build` / `go vet` / `gofmt` / `go test ./...` | PASS / PASS / 无输出 / 69 包 PASS |
| 授权专项测试（真实 MySQL） | **PASS**（所有者/私聊接收方/群成员/非参与者 2503/tombstone/退群/未知 fileID/文本不授权） |
| 图片代码 Linux Docker race | **PASS**（全量 `go test -race ./...`，exit 0，无 race warning） |
| 群聊图片 WS E2E（Chrome + Compose） | **PASS**（B/C 收到并预览；D 非成员 2503；B 退群后 2503；owner 仍可下载） |
| Web `vue-tsc` / `vitest` | PASS / 21 files 114 tests |

## 未验证（NOT RUN）

- 图片上传/下载吞吐与预览性能。
- 完整性能阶梯、soak/restart、文件吞吐、远端 CI（历史遗留）。

## Push

- 本轮修复/测试/文档 commit 推送至 `origin/main`；不 `--force`，不移动 tag。
