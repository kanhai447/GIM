# GIM 第九轮 — 打 gim-v1.2 tag + 普通文件消息 Phase 1

## 0. 前提与授权

第八轮已完成图片 Phase 1 收尾：授权专项测试、race、群聊图片 E2E 均 PASS；HEAD = origin/main = c97618f，工作区干净；gim-v1/gim-v1.1 未移动；未打新 tag。

本轮目标：

- 打 annotated tag gim-v1.2，指向图片 Phase 1 收尾的最终 commit，并 push。
- 实现普通文件消息（msg.type=3）完整闭环：协议 → 后端 → 前端 → 测试 → 文档 → 操作日志 → commit → push。
- 只做普通文件消息，不做语音、视频、回复、引用、撤回、@、图文混排、好友上线提醒、user_confs。

授权范围：

- 打 tag 并 push；不移动/删除 gim-v1、gim-v1.1。
- 文件消息复用 file_objects/user_files，复用图片消息的授权语义（所有者、私聊接收方、群成员、tombstone、退群、非参与者）；如发现文件消息需要不同授权语义，先停止问我。
- 新增配置项 FILE_MESSAGE_MAX_BYTES（默认值自定，写入 .env.example 与文档）。
- 修改 chat.send/group.send 解析、校验、持久化、ACK、推送、历史 DTO；修改前端消息组件与发送入口。
- 文档同步 docs/04/05/06/07、README.md、docs/validation-status.md、docs/known-issues.md。

明确不做：

- 不实现语音/视频/回复/引用/撤回/@/图文混排。
- 不改消息表结构、不新增表/列；文件消息复用 msg JSON。
- 不重构 Hub/Client、幂等、session、Presence、好友闭环、图片消息实现。
- 不放宽测试、不虚构结果、不 --force、不移动/删除已有 tag。
- 不在本轮打 gim-v2.0。

如果必须偏离，先停止，输出原因、影响、备选方案、推荐方案，等我决定。

## 1. 红线

- reference/ 只读，禁止复制。
- 不提交密钥、私钥、真实配置值。
- 日志不记录密码、JWT、Authorization、Cookie、Secret。
- 不虚构测试、commit、push、tag 结果；环境不可用记录 NOT RUN/SKIP。
- 每个可独立验收操作写 agent_logs/operations/day-12/；没有记录视为未完成。
- commit 前 git status、git diff --check、对应测试、确认无 .env/reference//密钥进入 staged。

## 2. 先落盘本轮指令

- 将本指令全文写入 agent_logs/prompts/round-9.md。
- 输出文件行数和前 5 行确认。
- 执行 git status、git log --oneline -n 5、git tag，确认工作区干净、HEAD = c97618f、gim-v1/gim-v1.1 存在。

## 3. 任务零：打 gim-v1.2 tag

- 确认 HEAD = origin/main = c97618f，工作区干净。
- 执行：

```bash
git tag -a gim-v1.2 -m "GIM V1.2: image message Phase 1 (protocol, backend, web, authorization, race, group E2E); file/voice/video/recall V2 pending"
git push origin main（如无新 commit 可跳过），git push origin gim-v1.2。
```

- `git ls-remote --tags origin` 确认远端同时有 gim-v1、gim-v1.1、gim-v1.2，且前两者 hash 未变。
- 操作日志 001_task-0_tag-gim-v1.2.md。
- 如果 push 失败，记录真实错误，保留本地 tag，不 --force，不阻塞后续任务（但需在最终汇总中注明）。

## 4. 任务一：文件消息协议设计（文档先行）

### 4.1 阅读

- `server/internal/chat/message/`、`server/internal/group/message/` 中图片消息的实现（type=2）。
- `server/internal/file/` 中上传、file_objects/user_files、下载授权。
- `web/src/components/chat/MessageBubble.vue`、两个 Composer、stores/chat.ts/group.ts/chat_reconcile.ts。
- docs/06 图片消息小节、docs/05 文件下载授权小节。

### 4.2 设计

在 docs/06_WEBSOCKET_PROTOCOL.md 新增“普通文件消息”小节：

- `msg.type = 3`。
- payload 建议：

```json
{"type":3,"fileMsg":{"fileID":123,"url":"/api/file/123","fileName":"report.pdf","size":123456,"mimeType":"application/pdf"}}
```

字段名与图片消息风格一致；fileID 为 user_files.id，服务端校验归属。

- 校验：归属 → 非图片也允许（与图片区别）→ 大小 ≤ FILE_MESSAGE_MAX_BYTES → 参数。
- 错误码：Chat/Group 各自分配新码，写入错误码表，不冲突。
- DTO：历史与 WS 同构，只存元数据，不存二进制。
- 授权：复用图片消息的下载授权语义；如文件类型需要不同策略（例如可执行文件禁止上传），先停止问我。
- 兼容性：type=1/2 不变；旧客户端收到 type=3 显示占位。

更新 docs/05：说明文件上传与消息引用两步；新增错误码；下载授权沿用图片小节，补充“文件类型不限制 MIME 前缀，但大小受 FILE_MESSAGE_MAX_BYTES 限制”。

### 4.3 验收

- 协议文档先于代码提交。
- 操作日志记录设计决策。
- 如发现现有结构无法容纳文件消息，停止报告。

## 5. 任务二：后端实现

- 解析 type=3，校验 fileID 归属、大小、参数；失败不落库、不 ACK。
- 与文本/图片同一事务：insert message + 更新 session/unread；clientMsgId 幂等不变。
- ACK 只在事务成功后返回；推送与历史 DTO 同构。
- 复用图片消息的授权查询；如文件消息需要额外授权条件，先停止问我。
- 测试：解析、归属、大小、幂等、事务回滚；集成测试（环境可用则跑）发送文件消息 → ACK → 接收方收到 → 历史可查 → 重复 clientMsgId 不重复入库 → 非参与者下载被拒。
- go build/vet/gofmt/test PASS。

## 6. 任务三：前端实现

- Composer 增加“发送文件”入口；选择文件 → 调用现有文件上传 API → 获取 fileID/fileName/size/mimeType → 发送 type=3。
- 复用 clientMsgId、pending、ACK、重试、reconcile。
- 消息列表渲染文件消息：文件名、大小、下载按钮；下载走带 token 的 fileApi.download，不使用裸 `<a href>`。
- 加载/下载失败显示占位与重试；不使用未清洗 v-html。
- 私聊/群聊共用文件消息组件。
- vitest、vue-tsc、vite build PASS；E2E 环境可用则加“发文件 → 接收方下载”，不可用记录 NOT RUN。

## 7. 任务四：文档与验证状态

- docs/06：文件消息事件、payload、DTO、错误码、兼容性。
- docs/05：上传与消息引用、错误码、下载授权补充。
- docs/07：新增“文件消息发送”流程。
- docs/04：msg JSON type=3 结构、无新表/列、preview [文件]。
- README.md：V2 进度，明确“图片 + 普通文件已实现；语音/视频/撤回/@ 仍待”。
- docs/validation-status.md：文件消息单测/集成/前端/E2E 状态。
- docs/known-issues.md：更新“仅文本/图片”为“文本/图片/文件”；保留其他 V2 项。

## 8. 执行顺序与 commit

```text
落盘 round-9.md
  -> 任务零打 tag + push -> 操作日志
  -> 任务一协议设计 -> commit（docs: define file message protocol）
  -> 任务二后端 -> 测试 -> commit（feat(chat): support file messages）
  -> 任务三前端 -> 测试 -> commit（feat(web): send and render file messages）
  -> 任务四文档与验证状态 -> commit（docs: record file message support and validation）
  -> 全量测试 -> push
```

每个 commit 前：git status、git diff --check、对应测试、确认操作日志存在、确认无 .env/reference//密钥进入 staged。

不压成一个 commit；本轮不打 gim-v2.0。

## 9. 本轮结束输出

- tag gim-v1.2 是否创建并推送成功；远端三个 tag 的 hash。
- 文件消息协议摘要：type、payload、错误码、授权、兼容性。
- 后端修改文件与测试真实结果（PASS/FAIL/SKIP/NOT RUN）。
- 前端修改文件与测试真实结果。
- 集成/E2E 是否执行，结果或原因。
- 文档修改清单。
- 操作日志清单。
- commit hash 与 push 结果。
- 未验证项与剩余问题。
- 下一步建议：撤回 / 好友上线提醒 / user_confs / 文件消息性能。

## 10. 遇到以下情况停止并问我

- 文件消息需要与图片消息不同的下载授权语义（例如可执行文件特殊处理）。
- 需要新增表/列/索引才能实现文件消息。
- 现有文件上传返回的 metadata 不足以支撑文件消息（缺 fileID/fileName/size/mimeType）。
- 集成/E2E 环境不可用，是否允许以“单测 + 前端测试 PASS，集成 NOT RUN”收尾。
- tag push 失败（认证/网络/non-fast-forward）。
- 测试发现业务缺陷，修复超出文件消息范围。
