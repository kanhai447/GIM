# Day 10 — V2 Phase 1：图片消息（2026-11-12）

## 今日目标

实现图片消息完整闭环（协议 → 后端 → 前端 → 测试 → 文档 → commit → push），只做图片；不新增表/字段，不改 Gateway/Auth/Presence/好友闭环。

## 真实结果

| 项目 | 结果 | 证据 |
|---|---|---|
| 协议设计 | 完成 | `docs/06` §5.1–5.4、`docs/05` §6/§7；type=2 + `imageMsg`，错误码 2211–2214 / 2412–2415 |
| 后端构建/单测 | **PASS** | `go build` / `go vet` / `gofmt` / `go test ./...`（69 packages） |
| 聊天图片集成（真实 MySQL/Redis/WS） | **PASS** | ACK、接收、历史、重复 clientMsgId 仅 1 行、归属/MIME/大小错误码、接收方下载授权 |
| 群聊图片 | PASS（单测） | 归一化、幂等冲突、2412–2415；未单独跑群聊图片 WS E2E |
| Web 类型/测试/构建 | **PASS** | `vue-tsc`；`vitest` 21 files / 114 tests；`vite build` |
| 浏览器图片 E2E | **PASS** | Chrome + Compose 17 services；10 步（注册/好友/上传/发送/接收/预览/关闭） |
| 图片代码 race | **NOT RUN** | race 在图片功能之前执行，图片代码未单独复跑 |
| 群聊图片 E2E / 图片性能 | **NOT RUN** | 本轮未做 |

## 关键过程

1. 阅读现有文本消息/文件/前端实现，确认 `msg` JSON 可容纳 `imageMsg`、上传响应已含 `id/mimeType/size`，无需改 schema。
2. 协议先行并单独提交（`1997f58`）。
3. 后端：`ImageRepository.GetOwnedImage`（user_files JOIN file_objects）、服务端归一化、类型/错误码、配置 `IMAGE_MESSAGE_MAX_BYTES`、单元 + 真实 MySQL/WS 集成测试（`e2bdf36`）。
4. 前端：类型联合、WS 解析、`uploadImage`、`sendImage`、共享 `MessageBubble`（带 token 下载 Blob、预览、失败重试）、输入框“图片”按钮（`b1c1328`）。
5. 浏览器 E2E 首轮失败：接收方没有发送者的 `user_files` 引用，`GET /api/file/:id` 返回 2503。最小修复：文件服务允许“消息参与者”下载被图片消息引用的文件；非参与者仍拒绝（`a1ecf64`），E2E 重跑 PASS。
6. 文档/README/验证状态/known-issues 更新并提交（`35b640a`）。

## 修改文件

- 协议/文档：`docs/06`、`docs/05`、`docs/07`、`docs/04`、`README.md`、`docs/validation-status.md`、`docs/known-issues.md`。
- 后端：`chat/protocol`、`group/ws/protocol`、两个 message service/repository、两个 config、`cmd/chat-api`、`cmd/group-api`、file 下载授权、`.env.example`/`.env.compose.example`、测试。
- 前端：类型、WS 解析、file API、两个 store、`MessageBubble`、两个消息列表、两个 composer、两个 view、会话 preview、测试。
- 操作日志：`agent_logs/operations/day-10/001–005`。

## 问题与修复

1. **接收方图片 403（真实缺陷）**：图片引用发送者 `user_files.id`，下载只允许所有者。修复为“所有者或消息参与者”，E2E 验证通过。
2. 集成测试 clientMsgId 含空格导致格式校验失败；改为安全字符后 PASS。
3. TS 联合类型导致旧测试直接访问 `textMsg` 报错；加 type guard。
4. `<img src>` 不携带 Authorization；改为带 token 的 Blob 下载 + `URL.createObjectURL`。

## 当日验收

- 图片消息闭环完成：上传 → 发送 → 事务落库 → ACK → 推送 → 历史 → 渲染 → 预览。
- 无 schema 变更；未修改 Gateway/Auth/Presence/好友闭环；未打新 tag。
- 浏览器 E2E、集成测试、单测、类型检查、构建全部 PASS；NOT RUN 项如实记录。

## 下一步

- 建议先补图片代码 `-race` 与群聊图片 WS E2E，再进入下一 Phase（视频/文件/语音、回复/引用/撤回/@、好友上线提醒、`user_confs`）。
