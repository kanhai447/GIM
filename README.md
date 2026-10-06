# GIM Agent 最终执行文档（三仓库复审版）

本包以三个实际参考源码为准：FIM Server、用户 Web、Admin。GIM 采用“参考驱动的重新实现”：reference 只读，Agent 在全新的 `server/web/admin` 中自主编写，不以复制/改名方式迁移原代码。

优先阅读顺序：

1. `AGENTS.md`
2. `FINAL_AGENT_INSTRUCTION.md`
3. `docs/00_DECISIONS.md`
4. `docs/15_SOURCE_REAUDIT.md`
5. `docs/01_PROJECT_ARCHITECTURE.md`
6. `docs/09_IMPLEMENTATION_PLAN.md`
7. `docs/12_FOUR_DAY_PLAN.md`
8. 其余协议/数据库/API/测试/Git 文档
9. `docs/16_REFERENCE_IMPLEMENTATION_RULES.md` 明确如何参考原项目而不直接复制
10. `START_AGENT.md` 用于第一次和 Agent 对话

推荐新项目目录：`server/ + web/ + admin/`；三个 FIM 源码放在本地 `reference/` 作为只读参考。
