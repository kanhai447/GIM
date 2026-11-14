# GIM 第八轮 — 图片消息 Phase 1 收尾（race + 群聊图片 E2E + 文件授权专项测试）

## 0. 前提与授权

第七轮已完成：图片消息协议、后端、前端、私聊图片集成测试、浏览器图片 E2E 均 PASS；HEAD = origin/main = c590f9c，工作区干净；gim-v1/gim-v1.1 未移动；未打新 tag。

第七轮 E2E 发现并修复了真实缺陷：接收方无 user_files 引用导致下载 2503；修复 a1ecf64 将下载权限扩展为“所有者或引用该文件的聊天/群聊消息参与者”。该修复引入跨表 JSON 授权查询，本轮必须专项审计与测试。

本轮目标：补齐图片消息 Phase 1 的验证缺口，明确文件下载授权语义，不进入下一 V2 Phase，不打 tag。

授权范围：

- 补 go test -race 覆盖图片消息相关包（Linux 容器优先）。
- 补群聊图片 WS E2E（真实 Gateway + WS + 文件下载）。
- 审计并测试文件下载授权：消息参与者、tombstone、退群、非参与者、文本消息不触发查询。
- 按第 3 节推荐语义修正授权查询（仅限最小修改）；如涉及 schema 或产品语义冲突，先停止问我。
- 更新 docs/05、docs/06、docs/07、docs/known-issues.md、docs/validation-status.md、README.md。
- 操作日志 agent_logs/operations/day-11/；commit + push。

明确不做：

- 不实现普通文件消息、语音、视频、回复、引用、撤回、@、图文混排、好友上线提醒、user_confs。
- 不改消息表结构、不新增表/列；授权查询只读现有 chat_messages/group_messages/group_members/user_files/file_objects。
- 不重构 Hub/Client、幂等、session、Presence、好友闭环。
- 不放宽测试、不虚构结果、不 --force、不移动/删除 tag、不打新 tag。

如果必须偏离（schema 变更、产品语义冲突、大范围重构），先停止，输出原因、影响、备选方案、推荐方案，等我决定。

## 1. 红线

- reference/ 只读，禁止复制。
- 不提交密钥、私钥、真实配置值。
- 日志不记录密码、JWT、Authorization、Cookie、Secret。
- 不虚构测试、commit、push 结果；环境不可用记录 NOT RUN/SKIP。
- 每个可独立验收操作写 agent_logs/operations/day-11/；没有记录视为未完成。
- commit 前 git status、git diff --check、对应测试、确认无 .env/reference//密钥进入 staged。
- 授权查询修改必须附带安全测试；不能只改代码不测。

## 2. 先落盘本轮指令

- 将本指令全文写入 agent_logs/prompts/round-8.md。
- 输出文件行数和前 5 行确认。
- 执行 git status、git log --oneline -n 5、git tag，确认工作区干净、HEAD = c590f9c、两个 tag 存在。

## 3. 任务一：审计文件下载授权并明确语义

### 3.1 先审计当前实现

阅读 server/internal/file/ 中下载授权相关代码（尤其是 a1ecf64 修改的 repository/service/handler），回答：

- 私聊图片下载时，授权条件具体是什么？是否包含：
  - 当前用户是 user_files.user_id；
  - 或当前用户是引用该文件的 chat_messages 的 send_user_id / rev_user_id；
  - 是否排除 admin_deleted_at IS NOT NULL 的 tombstone 消息？
- 群聊图片下载时，授权条件是否包含：
  - 当前用户当前是 group_members 成员；
  - 是否排除已退群/被踢用户；
  - 是否排除 tombstone 群消息？
- 文本消息路径是否会误触发文件授权查询？
- 是否存在通过构造 fileID 探测非参与者文件的路径？查询是否带 LIMIT 1、是否有速率限制？

把审计结果写入操作日志，不要先改代码。

### 3.2 推荐授权语义（按此实现，除非冲突）

- 所有者：始终可下载自己的 user_files。
- 私聊接收方：当前用户是引用该文件的消息的 send_user_id 或 rev_user_id，且消息未被 tombstone（admin_deleted_at IS NULL），可下载。
- 群聊成员：当前用户当前是 group_members 成员，且引用该文件的消息未被 tombstone，可下载。
- 退群/被踢：不再是群成员，拒绝下载该群历史图片（除非同时是文件所有者）。
- tombstone 消息：消息被管理员删除后，接收方/群成员不能再通过该消息授权下载；所有者仍可下载。
- 非参与者：拒绝，返回统一 2503，不泄露文件是否存在。
- 文本消息：不触发文件授权查询。
- 撤回：V1 未实现撤回，本轮不处理；在 docs/known-issues.md 标注“撤回后的文件授权语义待 V2 定义”。

### 3.3 如果当前实现与推荐语义不一致

- 若只是缺少 tombstone/退群过滤，做最小修改：在授权查询中加条件，不改表结构。
- 若需要新增索引才能避免全表扫描，先停止报告，不要擅自加 migration。
- 若当前实现已经满足推荐语义，只补测试和文档。
- 任何修改都必须有对应安全测试。

### 3.4 验收

- 审计结论写入操作日志。
- 授权语义写入 docs/05_HTTP_API.md（下载授权小节）和 docs/07_CORE_FLOWS.md（图片消息流程补充授权）。
- 如做了代码修改，go build/vet/test PASS，且新增测试覆盖第 3.2 节各条。

## 4. 任务二：文件授权专项测试

新增测试（单元/集成，按环境可用性）：

- 所有者下载：PASS。
- 私聊接收方下载：发送者发图片，接收方可下载。
- 私聊 tombstone 后：管理员删除该消息后，接收方下载被拒；所有者仍可下载。
- 群成员下载：群成员可下载群图片。
- 退群后下载：退群用户下载该群历史图片被拒。
- 非参与者构造 fileID：返回统一 2503，不泄露存在性。
- 文本消息不触发文件授权查询：可通过 mock/查询计数或代码路径断言。
- 群聊非成员：未入群用户下载群图片被拒。

如果某条因环境不可用无法跑，记录 NOT RUN 及原因，不得标 PASS。

## 5. 任务三：补 race

优先 Linux 容器：go test -race ./... -count=1，覆盖至少 server/internal/chat/...、server/internal/group/...、server/internal/file/...、server/internal/user/friend/...。

记录镜像、命令、exit code、是否有 WARNING: DATA RACE。

如果环境不可用，记录 NOT RUN 及真实原因；不得用 Windows native 的失败冒充 race 结论。

若 race 发现 DATA RACE，先停止并报告，不要擅自扩大修复范围。

## 6. 任务四：群聊图片 WS E2E

在 Compose 全栈可用时执行；不可用则记录 NOT RUN。

步骤：

- 注册 A、B、C；
- A 建群并邀请 B、C；
- A 上传图片并通过 group.send 发送 type=2；
- B、C 收到 group.message，可下载并预览；
- 非群成员 D 下载该图片被拒（2503）；
- A 退群后下载该群历史图片被拒（按第 3.2 节语义）；
- 记录每步真实输出（脱敏）。

如果 E2E 发现缺陷，先停止报告根因与最小修复方案，等我确认后再改；不要顺手改其他逻辑。

## 7. 任务五：文档与验证状态

- docs/05_HTTP_API.md：新增“文件下载授权”小节，写明所有者/私聊接收方/群成员/tombstone/退群/非参与者语义。
- docs/06_WEBSOCKET_PROTOCOL.md：图片消息小节补充“下载授权依赖文件服务，消息 DTO 不含二进制”。
- docs/07_CORE_FLOWS.md：图片消息流程补充授权检查步骤。
- docs/known-issues.md：更新/新增：
  - 撤回后的文件授权语义待 V2；
  - JSON_EXTRACT 无索引的下载路径风险与缓解（仅授权查询、统一 2503、可后续加索引）；
  - 群聊图片 E2E 状态。
- docs/validation-status.md：更新 race、群聊图片 E2E、文件授权专项测试的真实结果。
- README.md：V2 Phase 1 验证状态更新，明确“图片 Phase 1 收尾”范围。
- 操作日志记录文档变更。

## 8. 执行顺序与 commit

```text
落盘 round-8.md
  -> 任务一授权审计（只读）-> 操作日志
  -> 如需要最小修改：修改 + 授权测试 -> commit（fix(file): tighten image download authorization）
  -> 任务二授权专项测试 -> commit（test(file): cover download authorization semantics）
  -> 任务三 race -> 操作日志（如有修复则 commit）
  -> 任务四群聊图片 E2E -> 操作日志（如有修复则 commit）
  -> 任务五文档与验证状态 -> commit（docs: record image phase authorization and validation）
  -> push
```

每个 commit 前：git status、git diff --check、对应测试、确认操作日志存在、确认无 .env/reference//密钥进入 staged。

不压成一个 commit；本轮不打 tag。

## 9. 本轮结束输出

- 已完成/未完成。
- 授权审计结论：当前实现与推荐语义的差异。
- 授权代码修改（如有）：文件、查询条件、测试结果。
- 授权专项测试：每条 PASS/FAIL/SKIP/NOT RUN + 证据。
- race：PASS/FAIL/NOT RUN + 环境。
- 群聊图片 E2E：PASS/FAIL/NOT RUN + 步骤摘要。
- 文档修改清单。
- 操作日志清单。
- commit hash 与 push 结果。
- 未验证项与剩余问题。
- 下一步建议：是否打 gim-v1.2 / gim-v2.0 tag，或进入下一 V2 Phase。

## 10. 遇到以下情况停止并问我

- 推荐授权语义与现有产品语义冲突（例如业务上要求退群后仍可下载）。
- 需要新增索引或 migration 才能满足授权查询性能。
- 授权修改需要改消息表结构或文件表结构。
- race 发现 DATA RACE，修复超出图片消息范围。
- 群聊图片 E2E 发现缺陷，修复涉及 Hub/广播/session 重构。
- 环境不可用，是否允许本轮以“授权专项测试 + 文档 PASS，race/E2E NOT RUN”收尾。
- push 失败（认证/网络/non-fast-forward）。
