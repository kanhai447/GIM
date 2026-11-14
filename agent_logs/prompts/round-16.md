# GIM 第十六轮 — 补推 + 修 A4 错误码重叠 + 澄清 A7 E2E 证据链 + 文档修正

## 0. 前提与授权

第十五轮已完成：docs/architecture-deep-dive.md（10 章、约 655 行）已生成，发现 A1–A11 差异；3 个 docs commit 仅本地，HEAD = 26bf39b，origin/main = 2d849bc；未打 tag；未改业务代码。

本轮目标：

- 网络恢复后补推 3 个 docs commit。
- 修 A4：chat/message/admin 错误码与撤回错误码数值重叠，分配独立错误码段。
- 澄清 A7：E2E 证据链——脚本入库，或如实标注“临时执行未入库”。
- 文档修正 A8、A9、A11：migration README、schema_migrations、docs/04 列名与 migration 对齐。
- A1/A2/A3/A5/A6/A10 统一写入 docs/known-issues.md，不逐个修代码。

本轮明确不做：

- 不清理死代码（A1/A2 的 deletion 表、HideSession 接线）。
- 不补 Group 消息仓库 1213 重试（A5）。
- 不改注册流程初始化 user_confs（A3）。
- 不实现全员禁言写入入口（A10）。
- 不实现 /api/auth/open_login 路由（A6）。
- 不加新功能、不打 tag、不改 v2-scope.md 的范围声明。
- 不放宽测试、不虚构结果、不 --force。

如果必须偏离（例如 A4 错误码修改影响前端映射或已有测试断言），先停止，输出原因、影响、备选方案、推荐方案，等我决定。

## 1. 红线

- reference/ 只读，禁止复制。
- 不提交密钥、私钥、真实配置值。
- 日志不记录密码、JWT、Authorization、Cookie、Secret。
- 不虚构测试、commit、push 结果；push 失败记录真实错误。
- 每个可独立验收操作写 agent_logs/operations/day-19/；没有记录视为未完成。
- commit 前 git status、git diff --check、对应测试、确认无 .env/reference//密钥进入 staged。
- 错误码修改必须前后端同步：后端定义、前端映射、文档错误码表、测试断言全部更新。

## 2. 先落盘本轮指令

- 将本指令全文写入 agent_logs/prompts/round-16.md。
- 输出文件行数和前 5 行确认。
- 执行 git status、git log --oneline -n 6、git log --oneline origin/main..HEAD、git tag，确认：
  - 工作区干净；
  - HEAD = 26bf39b；
  - 待推送为 3 个 docs commit；
  - 六个 tag 存在。

## 3. 任务一：补推 3 个 docs commit

- `git fetch origin`，确认远端无新提交；如有，先报告差异，不自动 merge/rebase。
- `git push origin main`。
- 记录真实结果：
  - 成功：记录 old..new main -> main；
  - 失败：记录完整错误，不 --force，不阻塞后续任务（但需在最终汇总注明）。
- 确认 `git log --oneline origin/main..HEAD` 为空，或记录失败状态。
- 操作日志 `001_task-1_push-docs.md`。

## 4. 任务二：修 A4 错误码重叠

### 4.1 先审计

- 读 `server/internal/chat/message/admin/` 中错误码定义（2220/2221/2222 或其他实际值），列出每个码的语义和产生位置。
- 读 `server/internal/chat/message/` 中撤回错误码（2218–2222），列出每个码的语义。
- 读 `server/internal/group/message/admin/` 和 `server/internal/group/message/` 的群聊错误码，确认 Group 侧是否也有重叠（报告提到 2419–2423，需核对 admin 是否也在其中）。
- 读 `web/src/` 和 `admin/src/` 中错误码映射（如有），确认前端依赖哪些码。
- 读 `docs/05_HTTP_API.md` 错误码表，确认已分配的段。
- 把审计结果写入操作日志，先不改代码。

### 4.2 分配独立错误码段

- 建议给 Admin 删除/管理操作分配独立段，例如 Chat Admin 2230–2239、Group Admin 2430–2439；如已有其他 Admin 码段，沿用并扩展。
- 不与以下段冲突：
  - 撤回 Chat 2218–2222、Group 2419–2423；
  - 图片 Chat 2211–2214、Group 2412–2415；
  - 文件 Chat 2215–2217、Group 2416–2418；
  - 好友 1113–1119；
  - 现有 Admin 1110–1112。
- 在 docs/05_HTTP_API.md 错误码表中新增/调整段，注明“Chat Admin”“Group Admin”。

### 4.3 修改

- 后端：替换 Admin 错误码常量，确认所有引用点同步。
- 前端：如有错误码映射，更新。
- 测试：更新断言；如果测试断言的是“错误码数值”，改为断言语义或新常量。
- 文档：docs/05 错误码表、docs/architecture-deep-dive.md 第 4 章 Admin 小节（如引用了旧码）。
- 不修改撤回错误码本身。

### 4.4 验收

- 全仓 rg 确认旧重叠码不再出现在 Admin 上下文。
- go build/vet/test PASS。
- 前端 vue-tsc/vitest（web/admin）PASS。
- 操作日志记录新旧码映射表、修改文件、测试结果。

## 5. 任务三：澄清 A7 E2E 证据链

### 5.1 先核实

- 检查 `web/e2e/`、`admin/e2e/`、`scripts/`、`server/tests/` 中是否有好友/图片/文件/撤回/Presence 的 E2E 脚本。
- 对照 `docs/validation-status.md` 中声称的 E2E 项，逐项确认“脚本是否在仓库”。
- 检查 `agent_logs/operations/day-6/`、`day-10/`、`day-11/`、`day-13/`、`day-16/` 中 E2E 操作记录，确认当时执行方式（临时脚本 / 入库脚本）。
- 把核实结果写入操作日志。

### 5.2 处理（二选一，按实际情况）

**情况 A：脚本还在临时目录或操作日志中有完整命令**

- 整理进 `web/e2e/` 或 `scripts/e2e/`，命名清晰（如 `friend-flow.e2e.mjs`、`image-message.e2e.mjs`、`file-message.e2e.mjs`、`recall.e2e.mjs`、`friend-presence.e2e.mjs`）。
- 添加 README 说明运行前提（Compose 全栈、`RECALL_WINDOW_SECONDS=3` 等）。
- 不修改 E2E 逻辑，只做整理入库。
- 在 docs/validation-status.md 中把对应 E2E 项的证据链接到脚本路径。

**情况 B：脚本已丢失或从未入库**

- 不得虚构“脚本已入库”。
- 在 docs/validation-status.md 中把对应 E2E 项标注为“临时脚本执行，未入库”，并说明：
  - 执行日期与环境；
  - 操作日志路径作为证据；
  - 如需复现，按 docs/ 步骤重写。
- 在 docs/known-issues.md 新增条目：“部分 E2E 脚本未入库，复现需按文档重写”。
- 不得混用：不要把“临时执行”写成“脚本已入库”，也不要把“脚本已入库”写成“临时执行”。

### 5.3 验收

- 仓库中 E2E 脚本清单与 validation-status.md 一致。
- 如入库，脚本路径、README、运行前提齐全。
- 如未入库，validation-status.md 和 known-issues.md 如实标注。
- 操作日志记录核实结果与处理方式。

## 6. 任务四：文档修正（A8、A9、A11）

### 6.1 A8：server/migrations/README.md

- 核对实际 migration 数量（应为 008）。
- 修正 README 中的“到 007”为“到 008”，补充 008 的用途（只加列不加索引）。
- 不改 migration 文件本身。

### 6.2 A9：schema_migrations 未列入 docs/04

- 在 docs/04_DATABASE_DESIGN.md 的“通用约定”或“系统表”小节补充 schema_migrations：
  - 字段、用途（migration runner 版本记录）、是否由业务代码读写（否）。
- 与 server/migrations/runner.go 的实际表结构一致。

### 6.3 A11：docs/04 列名与 migration 不一致（D1–D9）

- 以 `server/migrations/*.sql` 为准，逐表核对 docs/04 中的列名、类型、索引、唯一约束。
- 修正 docs/04，使与 migration 一致。
- 在 docs/04 顶部或修正处注明“以实现为准，核对日期 2026-11-14”。
- 不修改 migration 文件；如果发现 migration 本身有误（例如缺索引），先停止报告，不擅自改。

### 6.4 验收

- migrations/README.md 与实际一致。
- docs/04 含 schema_migrations。
- D1–D9 逐项核对完毕，差异清零或在文档中标注“待实现”。
- 操作日志记录逐项修正。

## 7. 任务五：其余差异写入 known-issues.md

把以下 A 项统一写入 docs/known-issues.md，格式与现有条目一致（编号、状态、说明、代码位置）：

| 编号 | 内容 | 状态 |
|---|---|---|
| A1 | chat_message_deletions / group_message_deletions 生产代码只读，无写入方；隐藏会话实际走 hidden_at | 未使用表/待清理 |
| A2 | DELETE /api/chat/chat 未找到 HTTP 入口；Service.HideSession 未接线 | 未实现 |
| A3 | 注册不初始化 user_confs；CanChat 用 COALESCE 容忍 | 已知边界 |
| A5 | Group 消息仓库无 1213 死锁重试（Chat 有 3 次） | 已知边界 |
| A6 | /api/auth/open_login 在 Auth Allowlist 但无路由；/api/settings/open_login 实际存在 | 文档/实现不一致，已标注 |
| A10 | groups.is_all_muted 只读，无写入入口 | 全员禁言未实现 |

要求：

- 不修改代码。
- 不删除已有 known-issues 条目。
- 每条注明“发现于第十五轮深度文档审计”。
- 与 docs/v2-scope.md 的“V2 后期/不做”不重复；如重复，引用 v2-scope 而不重复展开。

## 8. 执行顺序与 commit

```text
落盘 round-16.md
  -> 任务一补推 -> 操作日志（不新增 commit，除非需要记录 push 状态）
  -> 任务二 A4 错误码 -> 测试 -> commit（fix: separate admin error codes from recall）
  -> 任务三 A7 E2E 澄清 -> commit（docs: clarify e2e evidence or add scripts）
  -> 任务四 A8/A9/A11 文档修正 -> commit（docs: align database and migration docs）
  -> 任务五 known-issues -> commit（docs: record architecture audit findings）
  -> 全量测试 -> push
```

每个 commit 前：git status、git diff --check、对应测试、确认操作日志存在、确认无 .env/reference//密钥进入 staged。

不压成一个 commit；本轮不打 tag。

## 9. 本轮结束输出

- 补推结果：3 个 docs commit 是否推送成功；失败则记录真实错误。
- A4 审计结果与新旧错误码映射表；前后端与文档同步情况；测试结果。
- A7 核实结果：哪些 E2E 脚本在仓库、哪些是临时执行；处理方式（入库/标注）。
- A8/A9/A11 修正清单：逐项修改前后对比。
- A1/A2/A3/A5/A6/A10 写入 known-issues 的编号与位置。
- 实际测试命令与结果（PASS/FAIL/SKIP/NOT RUN）。
- 操作日志清单。
- commit hash 与 push 结果。
- 未验证项与剩余问题。
- 下一步建议：面试设计问答文档 / 是否清理 A1/A2 / 是否补 Group 1213 重试 / 性能。

## 10. 遇到以下情况停止并问我

- A4 错误码修改影响前端错误提示或已有集成测试断言，且修改范围超出“换常量”。
- Group Admin 错误码也存在重叠，需要额外段分配。
- A7 核实后发现部分 E2E 脚本存在但依赖已删除的临时环境，无法直接入库。
- A11 核对发现 docs/04 与 migration 冲突，且无法判断“以实现为准”是否安全（例如索引差异可能影响性能）。
- 发现 migration 本身有误（缺索引、列类型错误），需要改 migration。
- push 失败（认证/网络/non-fast-forward）。
- 全量测试出现 FAIL，且原因不属于本轮修改范围。
