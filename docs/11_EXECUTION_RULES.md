# 11 Agent 执行规则（4 天交付版）

本文件补充 `AGENTS.md`，用于约束 Agent 在约 4 天内分阶段完成 GIM。若与普通建议冲突，以 `AGENTS.md`、`docs/00_DECISIONS.md` 和本文件的强制规则为准。

## 1. 基本原则

1. 不允许一次性“全项目重写”。必须按 `docs/09_IMPLEMENTATION_PLAN.md` 的 Phase 顺序推进，并按 `docs/12_FOUR_DAY_PLAN.md` 映射到 Day 1–Day 4。
2. 一个 Phase 未通过验收，不得声称完成，也不得把依赖它的后续 Phase 标记为完成。
3. 每次修改前先说明目标、涉及文件、协议/表结构影响；修改后必须执行测试并记录结果。
4. 任何偏离已确认设计的改动，先停止编码，输出：现状、问题、候选方案、兼容性影响、推荐方案，等待用户决定。
5. 不得虚构运行、测试、压测、Git commit、GitHub push、数据库迁移或前端联调结果。
6. 不得为了赶进度删除核心验收项；如 4 天内存在风险，只能按照“降级优先级”处理非 V1 阻塞项，并在日志中明确记录。

## 2. 每个工作单元的固定闭环

每个 Phase 或可独立验收的子阶段必须遵循：

```text
Read -> Plan -> Change -> Build/Test -> Review Diff -> Write Operation Log -> Update Daily/Summary Log -> Commit -> Push checkpoint -> Stop/Continue
```

其中：

- Read：重新读取本阶段涉及的设计文档与原实现，只提取行为/接口/边界，不复制其业务实现。
- Plan：输出文件级修改计划。
- Change：只改当前阶段范围。
- Build/Test：执行本阶段规定的测试。
- Review Diff：检查 `git diff`，确认无无关重构、密钥、生成物。
- Write Operation Log：立即在 `agent_logs/operations/day-N/` 新增本次操作记录。
- Update Daily/Summary Log：同步更新 `agent_logs/AGENT_WORKLOG.md` 与当日 `agent_logs/day-N.md`。
- Commit：测试通过后提交。
- Push checkpoint：达到规定检查点且远端可用时推送。
- Stop/Continue：若用户要求逐阶段确认则停止；若用户已明确允许当天连续执行，则只能在同一天计划范围内继续。


## 2A. Reference 使用方式

每个模块执行顺序必须是：

```text
阅读 GIM 设计文档 -> 查看 FIM 对应功能行为/接口 -> 写出差异和自己的实现方案 -> 在 GIM 新文件中编码 -> 测试 -> 记录参考点与改进点
```

禁止把“复制原文件再修改”作为任何 Phase 的实施步骤。若 Agent 发现复制更快，也必须放弃该方式并按自主实现执行。

## 3. 操作日志是强制交付物

Agent 开工时创建：

```text
agent_logs/
  README.md
  AGENT_WORKLOG.md
  day-1.md
  day-2.md
  day-3.md
  day-4.md
  operations/
    day-1/
    day-2/
    day-3/
    day-4/
```

规则：

- `AGENT_WORKLOG.md` 为追加式总日志，不覆盖历史记录。
- `agent_logs/operations/` 为**逐操作记录目录**。每完成一个可独立验收操作，必须立即创建一份新的 Markdown 文件，禁止只在当天结束时补写。
- 操作记录命名建议：`NNN_<phase>_<short-action>.md`，例如 `agent_logs/operations/day-2/003_phase-2_add-chat-hub.md`。
- “可独立验收操作”包括但不限于：新增/修改一个核心模块、完成一次 migration、修复一个明确 bug、完成一组协议改造、完成一次测试/压测闭环、完成一次 Git checkpoint。
- 每份逐操作记录至少包含：时间、目标、修改文件、执行命令、关键结果、测试、问题/修复、对应 commit（如有）、push 状态（如有）、下一步。
- **没有逐操作日志文件 = 该操作未完成；不得在总日志、日总结或对用户回复中标记为完成。**
- `day-N.md` 记录当天详细工作。
- 每个 Phase 至少记录：开始时间、目标、修改文件、关键命令、测试结果、错误与修复、设计决定、commit hash、push 结果、遗留事项。
- 关键错误必须记录根因与修复，不得只写“已解决”。
- 日志可以概括重复命令，不要求记录 `ls/pwd` 等无价值操作。
- 日志中不得写入 JWT、密码、数据库密码、GitHub Token、Cookie、私钥或完整 `.env` 内容；敏感值统一写 `[REDACTED]`。
- 日志属于项目历史，默认纳入 Git 提交。

模板见 `docs/14_OPERATION_LOG_TEMPLATE.md`。

## 4. Git / GitHub 强制规则

详细规则见 `docs/13_GIT_GITHUB_WORKFLOW.md`。最低要求：

1. 修改前检查 `git status`、当前分支、remote。
2. 禁止 `git push --force`、`git reset --hard` 覆盖用户未提交工作、重写共享历史。
3. 禁止提交 `.env`、token、私钥、数据库数据目录、`node_modules`、构建缓存和大体积临时上传文件。
4. 每个可验收 Phase 至少一个语义清晰的 commit；禁止把 4 天全部压成一个 commit。
5. 每天至少建立一个可回滚 GitHub checkpoint；若当日存在多个关键阶段，可多次 push。
6. push 前必须先运行对应测试；测试失败的 commit 不得标记为稳定 checkpoint。
7. GitHub push 失败时记录真实错误并继续保留本地 commit，禁止写“已上传”。
8. 若仓库没有 remote 或没有认证权限，Agent 只做到本地 commit，并明确告诉用户需要配置的最小步骤；不得自行创建未知组织/仓库。

## 5. 代码与文档同步规则

以下变更必须同一 Phase 更新对应文档：

- 数据表/索引：`docs/04_DATABASE_DESIGN.md`
- HTTP 路由/字段/错误码：`docs/05_HTTP_API.md`
- WebSocket event/payload/ACK：`docs/06_WEBSOCKET_PROTOCOL.md`
- 核心流程：`docs/07_CORE_FLOWS.md`
- Redis key/TTL：`docs/08_REDIS_CONFIG.md`
- 实施范围变化：`docs/09_IMPLEMENTATION_PLAN.md` 与 `docs/12_FOUR_DAY_PLAN.md`
- 架构决策变化：只有用户确认后才能改 `docs/00_DECISIONS.md`

代码与文档不允许长期存在两套事实。

## 6. 4 天进度风险处理

优先级从高到低：

P0（不可删）：自主编写的可启动 GIM 基线、鉴权、好友、Chat/Group WebSocket、Hub/Client、心跳、消息持久化、ACK/幂等、历史消息、session/未读、核心文件上传、前后端可联调、测试和 README。

P1（尽量完成）：文件物理去重完整模型、更多消息类型 UI、复杂异常恢复、更多集成测试。

P2（允许延期）：WebRTC、WS Ticket、Gateway 本地 JWT、多实例 WS 路由、Kubernetes、复杂监控。Kafka/Logs 作为 Admin 完整链路安排在 Day 4；若 Day 4 时间不足，必须明确标记为未完成，不能把项目描述成已经具备安全异步日志链路。

若时间受限，Agent 只能延期 P2，必要时说明 P1 的未完成项；不得删除 P0 后声称“V1 完成”。

## 7. 每日结束定义（Definition of Done）

当天只有在以下条件均满足时才能标记完成：

- 当天计划中的核心验收项通过；
- 代码能够构建；
- 对应测试已实际运行；
- 日志已更新；
- `git status` 状态已解释清楚；
- 已创建 checkpoint commit；
- 若远端可用，已 push 到 GitHub，并记录 commit hash/branch；
- 列出次日入口和当前已知问题。
