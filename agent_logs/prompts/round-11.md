# GIM 第十一轮 — 撤回实现（后端 + 前端 + 测试 + 文档）

## 0. 前提与授权

第十轮已完成：文件 Phase 收尾（race + 群聊文件 E2E）PASS；撤回协议已在 docs/06 §5.6 定义；HEAD = origin/main = 0e0ff81，工作区干净；三个 tag 未移动；RECALL_WINDOW_SECONDS=120 已预留但尚未接线。

本轮目标：按 docs/06 §5.6 实现撤回消息（type=4）的完整闭环：后端解析/校验/落库/ACK/推送/历史替换/文件授权联动，前端撤回入口与占位替换，单测 + 集成 + E2E，文档从“设计”改为“已实现”。

授权范围：

- 后端：chat.send/group.send 支持 type=4；校验顺序按协议；落库撤回消息；历史 DTO 替换；文件授权查询排除被撤回原消息。
- 前端：时间窗口内显示撤回入口；撤回消息渲染；本地按 msgID 替换原消息。
- 配置：RECALL_WINDOW_SECONDS 接线到 chat/group 配置。
- 测试：单测、集成、E2E；覆盖协议 §5.6 所有校验分支和授权联动。
- 文档：docs/06 从“设计”改为“已实现”；docs/05/07、README.md、docs/validation-status.md、docs/known-issues.md 同步。
- 操作日志 agent_logs/operations/day-14/；commit + push。

明确不做：

- 不实现管理员撤回、好友上线提醒、user_confs、语音/视频/@/图文混排。
- 不新增表/列/索引；撤回复用 msg JSON，原消息行保留。
- 不重构 Hub/Client、幂等、session、Presence、好友闭环、图片/文件消息实现。
- 不放宽测试、不虚构结果、不 --force、不移动/删除 tag、不打新 tag。

如果必须偏离（例如需要新增列才能正确过滤历史、授权查询性能不可接受），先停止，输出原因、影响、备选方案、推荐方案，等我决定。

## 1. 红线

- reference/ 只读，禁止复制。
- 不提交密钥、私钥、真实配置值。
- 日志不记录密码、JWT、Authorization、Cookie、Secret。
- 不虚构测试、commit、push 结果；环境不可用记录 NOT RUN/SKIP。
- 每个可独立验收操作写 agent_logs/operations/day-14/；没有记录视为未完成。
- commit 前 git status、git diff --check、对应测试、确认无 .env/reference//密钥进入 staged。
- 授权查询修改必须附带安全测试；不能只改代码不测。

## 2. 先落盘本轮指令

- 将本指令全文写入 agent_logs/prompts/round-11.md。
- 输出文件行数和前 5 行确认。
- 执行 git status、git log --oneline -n 5、git tag，确认工作区干净、HEAD = 0e0ff81、三个 tag 存在。

## 3. 任务一：后端实现

### 3.1 解析与校验

在 chat.send/group.send 解析层支持 type=4，payload `{"type":4,"withdrawMsg":{"msgID":123}}`。

按 docs/06 §5.6 的校验顺序实现，顺序不得调换：

1. 原消息存在且属于当前会话（私聊：send_user_id/rev_user_id 包含当前用户；群聊：group_id 匹配）；
2. 当前用户是原消息发送者；
3. 原消息未被 tombstone（admin_deleted_at IS NULL）；
4. 原消息未被撤回（不存在指向它的 type=4 消息）；
5. 距原消息 createdAt 不超过 RECALL_WINDOW_SECONDS；
6. 群聊：当前用户仍是 group_members 成员。

校验失败返回协议定义的错误码（Chat 2218–2221、Group 2419–2422、群非成员 2404），不落库、不 ACK 成功。

参数缺失/非法 msgID 返回参数错误码。

### 3.2 持久化与幂等

- 撤回消息与文本/图片/文件消息走同一事务：insert type=4 消息 + 更新双方/群成员 session（preview 为 [撤回]）。
- 幂等：
  - 同一发送者、同一 msgID 重复撤回，返回同一 ACK，不重复插入；
  - 他人已撤回该消息，返回 2221/2422；
  - clientMsgId 幂等不变。
- ACK 只在事务成功后返回。
- 推送：chat.message/group.message 推送撤回消息；接收方/群成员按 msgID 替换本地原消息。

### 3.3 历史 DTO 替换

历史查询（GET /api/chat/history、GET /api/group/history/:id）对已被撤回的原消息：

- 替换为 type=4 占位，[撤回]，不返回原内容；
- 撤回消息本身在历史中可见（或按协议合并为一条占位，以 docs/06 §5.6 为准）。

原消息行保留在 DB（审计/幂等），只在读取侧替换。

确保历史分页、beforeId/afterId 游标不受影响。

测试断言：撤回后历史不包含原文本/图片/文件 payload。

### 3.4 文件授权联动

现有授权查询匹配 imageMsg.fileID 和 fileMsg.fileID。撤回后，原消息引用的文件不再授权非所有者下载。

实现方式：授权查询增加条件，排除“已被撤回的原消息”：

- 私聊：不存在指向该消息的 type=4 撤回消息；
- 群聊：同上，且当前用户仍是群成员。

所有者始终可下载自己的 user_files，不受撤回影响。

授权查询保持 `SELECT 1 ... LIMIT 1`，不改成 COUNT(*)。

新增安全测试：撤回后接收方/群成员下载被拒（2503），所有者仍可下载。

### 3.5 配置接线

RECALL_WINDOW_SECONDS 从 .env.example 接入 chat/group 配置，默认 120，测试环境可设短值。

配置缺失时有合理默认值，不 panic。

更新 .env.compose.example。

### 3.6 测试

- 单元测试：解析、校验顺序、时间窗口边界、幂等、非本人、已撤回、tombstone、群非成员。
- 集成测试（环境可用则跑）：
  - 发送文本/图片/文件 → 撤回 → ACK → 接收方收到撤回 → 历史替换 → 原内容不可见；
  - 超过窗口撤回被拒；
  - 非本人撤回被拒；
  - 重复撤回幂等；
  - 撤回后接收方下载文件被拒，所有者仍可下载。
- go build/vet/gofmt/test PASS。

## 4. 任务二：前端实现

### 4.1 撤回入口

- 在自己发送的消息上，时间窗口内显示“撤回”入口（右键菜单或悬浮按钮，按现有 UI 风格）。
- 超过窗口隐藏或禁用，前端仅作 UX 提示，服务端仍必须校验。
- 点击撤回 → 发送 type=4，复用 clientMsgId、pending、ACK、重试。
- 撤回失败显示错误提示，不静默。

### 4.2 渲染与本地替换

- 收到 type=4 撤回消息后，按 msgID 定位本地原消息，替换为“消息已撤回”占位；不保留原内容。
- 历史加载时，已被撤回的原消息直接渲染为占位。
- 撤回占位不显示原图片/文件卡片/下载按钮。
- 不使用未清洗 v-html。
- 私聊/群聊共用撤回占位组件。

### 4.3 测试

- vitest：撤回入口显示/隐藏、发送、ACK、本地替换、历史占位、失败提示。
- vue-tsc、vite build PASS。
- E2E（环境可用则跑）：
  - A 发文本/图片/文件 → A 撤回 → B 看到占位 → 原内容不可见；
  - 超过窗口撤回入口消失或失败；
  - 撤回后 B 下载原文件被拒。
- 不可用则记录 NOT RUN。

## 5. 任务三：文档与验证状态

- docs/06：§5.6 从“设计”改为“已实现”，补充实际错误码与历史 DTO 行为；如实现与设计有差异，写明差异与原因。
- docs/05：错误码表确认；下载授权小节补充“撤回后非所有者失效”已实现。
- docs/07：§2.3 撤回流程从“设计”改为“已实现”。
- docs/04：确认撤回复用 msg JSON、无新列；preview [撤回]。
- docs/known-issues：K-12 改为“已实现”；K-13 管理员撤回仍 V2；V-16 撤回实现移入已验证或更新状态；如授权查询 NOT EXISTS 有性能风险，记录并说明缓解。
- docs/validation-status：撤回单测/集成/前端/E2E 真实状态。
- README：V2 进度“图片 + 普通文件 + 撤回已实现；语音/视频/@/上线提醒/user_confs 待”。
- .env.example/.env.compose.example：RECALL_WINDOW_SECONDS 从“预留”改为“已接线”。

## 6. 执行顺序与 commit

```text
落盘 round-11.md
  -> 任务一后端 -> 测试 -> commit（feat(chat): support recall messages）
  -> 任务一授权联动 -> 安全测试 -> commit（fix(file): revoke download after recall）
  -> 任务二前端 -> 测试 -> commit（feat(web): recall messages）
  -> 任务三文档与验证状态 -> commit（docs: record recall support and validation）
  -> 全量测试 -> push
```

每个 commit 前：git status、git diff --check、对应测试、确认操作日志存在、确认无 .env/reference//密钥进入 staged。

不压成一个 commit；本轮不打 tag。

## 7. 本轮结束输出

- 已完成/未完成。
- 后端修改文件与测试真实结果（PASS/FAIL/SKIP/NOT RUN）。
- 授权联动修改与安全测试结果。
- 前端修改文件与测试真实结果。
- 集成/E2E 是否执行，结果或原因。
- 实现与协议 §5.6 的差异（如有）。
- 文档修改清单。
- 操作日志清单。
- commit hash 与 push 结果。
- 未验证项与剩余问题。
- 下一步建议：好友上线提醒 / user_confs / 性能 / 是否打 gim-v2.0。

## 8. 遇到以下情况停止并问我

- 撤回需要新增列/表/索引才能正确过滤历史或授权。
- 授权查询加 NOT EXISTS 后性能不可接受（例如全表扫描超出预期）。
- 历史 DTO 替换与分页游标冲突，需要改查询结构。
- 群聊撤回时发送者已退群的语义需要你决定（协议写“仍为成员”，但产品上可能需要讨论）。
- 集成/E2E 环境不可用，是否允许以“单测 + 前端测试 PASS，集成 NOT RUN”收尾。
- 测试发现业务缺陷，修复超出撤回范围。
- push 失败（认证/网络/non-fast-forward）。
