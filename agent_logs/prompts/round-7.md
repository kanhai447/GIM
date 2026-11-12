# GIM 第七轮 — V2 Phase 1：图片消息（仅图片，不做视频/语音/撤回/@）

## 0. 前提与授权

GIM V1.1 已交付：HEAD = origin/main = 78aa9c5，gim-v1 与 gim-v1.1 均已推送，远端一致。V1 范围是文本 IM + 文件上传 + 好友闭环 + Admin + 审计。

本轮目标：实现图片消息的完整闭环（协议 → 后端 → 前端 → 测试 → 文档 → 操作日志 → commit → push）。只做图片，不做视频、语音、普通文件消息、回复、引用、撤回、@、图文混排。

授权范围：

- 定义图片消息的 WS payload 与 HTTP DTO，写入 docs/06_WEBSOCKET_PROTOCOL.md。
- 后端：chat.send / group.send 支持图片消息；服务端校验文件归属、MIME、大小；事务落库；ACK；推送；历史 DTO 返回图片结构。
- 前端：聊天/群聊支持选择图片、上传、发送图片消息、渲染图片、点击预览。
- 测试：单元测试、集成测试、前端组件测试；E2E 可加一步“发图片”。
- 文档同步：docs/06、docs/05、docs/07、docs/04（如需）、README.md、docs/validation-status.md。
- 操作日志 agent_logs/operations/day-10/；commit + push。

明确不做：

- 不实现视频、语音、普通文件消息、回复、引用、撤回、@、图文混排。
- 不新增消息表；复用 chat_messages / group_messages 的 msg JSON 字段。
- 不新增文件表；复用 file_objects / user_files。
- 不改 Gateway/Auth 链路、不改 Presence、不改好友闭环。
- 不重构已通过 race 的 Hub/Client、幂等、session 实现。
- 不放宽测试、不虚构结果、不 --force、不移动/删除已有 tag。

如果必须偏离（例如需要新增字段、索引或表），先停止，输出：原因、影响、备选方案、推荐方案，等我决定。

## 1. 红线

- reference/ 只读，禁止复制。
- 不提交密钥、私钥、真实配置值。
- 日志不记录密码、JWT、Authorization、Cookie、Secret。
- 不虚构测试、commit、push 结果；环境不可用记录 NOT RUN/SKIP。
- 每个可独立验收操作写 agent_logs/operations/day-10/；没有记录视为未完成。
- 测试未通过不得 commit；commit 前 git status、git diff --check、确认无 .env/reference//密钥进入 staged。
- 删除/修改现有协议前，先在文档中写明兼容性影响。

## 2. 先落盘本轮指令

- 将本指令全文写入 agent_logs/prompts/round-7.md。
- 输出文件行数和前 5 行确认。
- 执行 git status、git log --oneline -n 5、git tag，确认工作区干净、HEAD = 78aa9c5、gim-v1 与 gim-v1.1 存在。

## 3. 任务一：阅读现有实现并设计图片消息协议（文档先行）

### 3.1 阅读

- server/internal/chat/message/：现有文本消息的解析、校验、持久化、ACK、推送、历史 DTO。
- server/internal/group/message/：群消息对应实现。
- server/internal/file/：上传、file_objects / user_files 模型、返回的 metadata、下载/预览路由。
- web/src/stores/chat.ts、group.ts、chat_reconcile.ts：消息发送、ACK、去重、渲染。
- web/src/views/ChatView.vue、GroupView.vue、消息组件：文本消息渲染方式。
- docs/06_WEBSOCKET_PROTOCOL.md、docs/05_HTTP_API.md、docs/04_DATABASE_DESIGN.md：现有消息类型与文件字段。

### 3.2 设计

在 docs/06_WEBSOCKET_PROTOCOL.md 新增“图片消息”小节，至少包含：

- msg.type 取值（建议 2，与现有文本 1 区分；若现有代码已有类型枚举，以现有为准并说明）。
- chat.send / group.send 的图片 payload 结构。建议：

```json
{
  "type": 2,
  "imageMsg": {
    "fileID": 123,
    "url": "/api/file/...",
    "width": 800,
    "height": 600,
    "size": 123456,
    "mimeType": "image/png"
  }
}
```

字段名以现有 DTO 风格为准；fileID 必须是 user_files.id，服务端据此校验归属。

- 历史 DTO 中图片消息的结构（与 WS 同构，避免前端两套解析）。
- ACK 与文本消息一致：事务成功后返回 messageId、createdAt。
- 错误码：文件不存在、文件不属于发送者、非图片 MIME、超过大小限制、参数缺失。
- 兼容性：现有文本消息 type=1 不受影响；旧客户端收到 type=2 时应能忽略或显示占位，不崩溃。

同时更新 docs/05_HTTP_API.md：说明图片消息引用的是已上传文件的 user_files.id，上传仍走现有 /api/file/image（或现有图片上传路由），上传与发消息是两个步骤。

### 3.3 验收

- 协议文档先于代码提交，包含 payload、DTO、错误码、兼容性说明。
- 操作日志记录设计决策与参考的现有代码路径。
- 如果发现现有消息结构无法容纳图片消息（例如 msg JSON 字段有严格 schema），停止并报告。

## 4. 任务二：后端实现

### 4.1 入站解析与校验

- 在 chat.send / group.send 的解析层支持 type=2 图片消息；未知类型返回明确错误，不 panic。
- 校验：
  - fileID 存在且属于发送者（查 user_files，user_id = 发送者）。
  - 对应 file_objects 的 mime_type 以 image/ 开头。
  - 文件大小不超过配置的图片上限（复用或新增配置项，写入 .env.example 与文档）。
  - width/height/size 可选，但若提供需为合法数值。
- 校验失败返回明确错误码，不落库、不 ACK 成功。

### 4.2 持久化与推送

- 图片消息与文本消息走同一事务：insert message + 更新双方 session/unread。
- clientMsgId 幂等不变：同一发送者同一 clientMsgId 只生成一条图片消息。
- ACK 只在事务成功后返回。
- 推送接收方/群成员时，chat.message / group.message 的 data 使用历史 DTO 同构结构。
- 历史查询 GET /api/chat/history、GET /api/group/history/:id 返回图片消息结构，不返回原始二进制。

### 4.3 测试

- 单元测试：图片消息解析、归属校验、MIME 校验、大小限制、幂等、事务回滚。
- 集成测试（如果环境可用）：发送图片消息 → ACK → 接收方收到 → 历史可查 → 重复 clientMsgId 不重复入库。
- 环境不可用则记录 NOT RUN，不虚构。

### 4.4 验收

- go build ./...、go vet ./...、gofmt -l server、go test ./... -count=1 PASS。
- 新增/修改的测试真实执行。
- 操作日志记录修改文件、测试结果、错误码。

## 5. 任务三：前端实现

### 5.1 发送

- 在 ChatView.vue / GroupView.vue 增加“发送图片”入口（按钮或粘贴/拖拽，按现有 UI 风格）。
- 选择图片后先调用现有图片上传 API，获取 fileID/url/metadata；上传失败提示，不发送消息。
- 上传成功后通过 chat.send / group.send 发送 type=2 图片消息，复用现有 clientMsgId、pending、ACK、重试逻辑。
- pending 状态显示缩略图或占位；ACK 后转为已发送；失败可重试，复用同一 clientMsgId。

### 5.2 渲染

- 消息列表根据 type 渲染：文本用现有组件，图片用新组件（缩略图 + 点击预览）。
- 图片预览使用现有 UI 库的预览组件或轻量实现；不使用未清洗 v-html。
- 加载失败显示占位与重试，不阻塞其他消息。
- 群聊与私聊共用图片消息组件，避免两套逻辑。

### 5.3 测试

- vitest：图片消息解析、发送流程（mock 上传 + WS）、渲染、失败占位。
- vue-tsc、vite build PASS。
- E2E（如果环境可用）：发图片 → 接收方看到图片 → 点击预览。不可用则记录 NOT RUN。

### 5.4 验收

- 前端测试真实 PASS。
- 不破坏现有文本消息、好友、文件页面。
- 操作日志记录组件与状态管理改动。

## 6. 任务四：文档与验证状态

- 更新 docs/06_WEBSOCKET_PROTOCOL.md：图片消息事件、payload、DTO、错误码、V1 文本兼容说明。
- 更新 docs/05_HTTP_API.md：图片上传与消息引用的关系；新增错误码写入错误码表。
- 更新 docs/07_CORE_FLOWS.md：新增“图片消息发送”流程（上传 → 发送 → ACK → 推送 → 历史）。
- 更新 docs/04_DATABASE_DESIGN.md：说明图片消息复用 msg JSON 与 user_files，无新表。
- 更新 README.md：V2 进度，明确“图片消息已实现；视频/语音/文件消息/撤回/@ 仍为 V2 后续”。
- 更新 docs/validation-status.md：图片消息的单测/集成/前端/E2E 状态，NOT RUN 如实标注。
- 更新 docs/known-issues.md：移除“仅文本消息”中的图片部分，保留其他富消息为未实现。

## 7. 执行顺序与 commit

```text
落盘 round-7.md
  -> 任务一协议设计（文档先行）-> commit（docs: define image message protocol）
  -> 任务二后端 -> 测试 -> commit（feat(chat): support image messages）
  -> 任务三前端 -> 测试 -> commit（feat(web): send and render image messages）
  -> 任务四文档与验证状态 -> commit（docs: record image message support and validation）
  -> 全量测试 -> push
```

每个 commit 前：git status、git diff --check、对应测试、确认操作日志存在、确认无 .env/reference//密钥进入 staged。

不压成一个 commit。本轮不打新 tag（tag 留到你确认 V2 Phase 1 整体验收后）。

## 8. 本轮结束输出

- 已完成/未完成。
- 协议设计摘要：msg.type、payload、DTO、错误码、兼容性。
- 后端修改文件与测试真实结果（PASS/FAIL/SKIP/NOT RUN）。
- 前端修改文件与测试真实结果。
- 集成/E2E 是否执行，结果或原因。
- 文档修改清单。
- 操作日志清单。
- commit hash 与 push 结果。
- 未验证项与剩余问题。
- 下一步建议：视频/语音/文件消息/撤回/@ 的优先级，或先补图片消息的 E2E/性能。

## 9. 遇到以下情况停止并问我

- 现有 msg JSON 结构无法容纳图片消息，需要改 schema 或新增表/字段。
- 现有文件上传返回的 metadata 不足以支撑图片消息（缺 fileID、MIME、尺寸）。
- 需要修改 Gateway/Auth、Presence、好友闭环才能实现图片消息。
- 集成/E2E 环境不可用，是否允许本轮以“单测 + 前端测试 PASS，集成 NOT RUN”收尾。
- 测试发现业务缺陷，修复超出图片消息范围。
- push 失败（认证/网络/non-fast-forward）。
