# Agent Operations Index

本目录保存逐操作记录。每次可独立验收的操作对应一个文件；没有记录的操作视为未完成。

## 目录与命名

```text
operations/
  pre-day1/     项目初始化前的准备操作（历史命名 01-…14-）
  day-1/ … day-N/
    NNN_<phase>_<action>.md
```

- `day-N` 从 Day 1 起按自然日编号；day-5 起用于 V1 之后的收敛轮次。
- `NNN` 在单个目录内唯一。day-4 因多条工作流并行，采用 checkpoint 作用域编号，重复数字见下方“Day 4 编号说明”。
- 每份记录使用统一文件名：`NNN_<scope>_<short-action>.md`，不使用空格与大写。

## 记录字段

每份操作记录应覆盖以下字段；历史记录曾使用中英混排标题，语义等价：

| 字段 | 常见标题 |
|---|---|
| 目标 | `本次目标` / `目标` / `Goal` |
| 修改文件 | `修改文件` / `新增/修改文件` / `Main Files` / `Files` |
| 命令 | `执行命令` / `Commands` / `关键操作` |
| 结果 | `测试结果` / `结果` / `Verification` |
| 测试 | `测试` / `Tests` |
| 问题 | `问题与根因` / `Problems / Fixes` / `问题` |
| commit | `Git` → `Commit hash` / `commit` |
| push | `Git` → `Push` / `push` |

day-5 之后的记录使用精简模板（目标、修改文件、命令与结果、测试、Git、下一步），问题字段在无问题时省略或写“无”。

## Day 4 编号说明

day-4 的记录按 checkpoint 分组编号，多个工作流并行导致前缀重复，**文件名保持不变**（避免破坏历史引用与 Git 路径），按 checkpoint 归组如下：

| checkpoint | 文件 |
|---|---|
| CP1 Kafka audit | `001_checkpoint-1_kafka-audit-foundation.md` |
| CP2 Admin | `002_checkpoint-2_admin-integration.md`、`002_checkpoint-2_admin-service-apis.md`、`003_checkpoint-2_admin-frontend-integration.md`、`004_checkpoint-2_browser-e2e-interrupted.md`、`005_checkpoint-2_local-e2e-account-check.md`、`006_checkpoint-2_manual-browser-environment-start.md`、`007_checkpoint-2_fix-admin-login-controls.md`、`008_checkpoint-2_fix-user-status-action.md` |
| CP3 Compose | `003_checkpoint-3_docker-compose.md` |
| CP4 性能/稳定性 | `004_checkpoint-4_performance-stability-race.md` |
| CP5 最终验收 | `005_checkpoint-5_final-acceptance.md` |

## 事实修正

- 只修正事实错误（计数、错误归因、失效路径），不删除历史记录。
- 修正处标注“修正于 day-7”（或对应日期），保留原文可追溯。
- 已修正：Compose service 数（原记 19/18，实际删除 group-rpc 前 18、删除后 17）；见 `day-5.md`、`operations/day-5/005_task-4_remove-group-rpc.md`、`day-6.md`。
- “16 个长期服务”指 Compose 中除 `migration`/`storage-init` 两个一次性 job 外的 16 个常驻服务，与 18/17 的 service 总数不矛盾，未修改。

## 索引

- Day 1–4：`day-N.md` 汇总 + `operations/day-N/` 逐操作记录。
- Day 5：V1 收敛（C1/C2、好友闭环、文档、删除 group-rpc）。
- Day 6：补推 + 真实依赖验证 + 拒绝再申请语义 + 清理。
- Day 7：文档一致性、README 重写、代码整洁、日志结构、Git 整洁。
- `AGENT_WORKLOG.md`：跨轮追加式总日志；`CURRENT_STATE.md`：只写当前状态。
