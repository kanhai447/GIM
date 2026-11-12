# 004 / Task 4 — agent_logs 结构统一

- **日期**：2026-11-12（Asia/Shanghai）
- **开始状态**：`HEAD = 6b6a03d`。
- **结束状态**：本任务提交 `docs(logs): normalize operation logs`。

## 目标

统一 `agent_logs/` 的目录/编号/字段约定，修正历史误计，并确保 `CURRENT_STATE.md` 只写当前状态、不承担历史叙事。

## 结构盘点（真实）

```text
operations/pre-day1/  14 份（初始化准备，历史命名 01-…14-）
operations/day-1/     10 份
operations/day-2/      8 份
operations/day-3/      6 份
operations/day-4/     12 份（checkpoint 作用域编号，存在重复前缀）
operations/day-5/      5 份
operations/day-6/      4 份
operations/day-7/      3 份（本任务新增第 4 份）
```

## 发现与处理

1. **目录命名**：`pre-day1` 与 `day-1..day-7` 一致使用 kebab/连字符，无异常；未重命名历史目录。
2. **day-4 编号重复**：`002/003/004/005` 各有两条（按 checkpoint 并行编号）。**决定不重命名**，避免破坏 `docs/interview` 等既有链接与历史路径；改为在 `operations/README.md` 中给出 checkpoint→文件 的完整映射，并说明该目录接受 checkpoint 作用域编号。
3. **字段覆盖**：历史记录使用中英混排标题（`本次目标`/`Goal`、`修改文件`/`Files`、`执行命令`/`Commands`、`测试结果`/`Verification`、`问题与根因`/`Problems`、`Git` 内 commit/push）。在索引中给出标题对照表；对 day-6、day-7 的记录补齐显式 `目标` / `问题与修复` 段，使新记录符合模板。
4. **历史误计**：Compose service 数已在 day-6 修正为“删除前 18、删除后 17”（原 19/18），`day-5.md` 与 `operations/day-5/005...` 已标注“修正于 day-6”。本轮复核：
   - “16 个长期服务”= 18/17 service 中除 `migration`/`storage-init` 外的常驻服务，**不是误计**，保留。
   - `pre-day1/07` 的 “34 committed paths”、`AGENT_WORKLOG` 的 commit 描述均与上下文一致，无需修改。
5. **CURRENT_STATE.md**：已只保留当前状态（day-6 版本）；本轮收尾会更新为 day-7 状态，不写入历史流水。

## 新增/修改文件

| 文件 | 变更 |
|---|---|
| `agent_logs/operations/README.md` | 新建：目录/命名约定、字段与标题对照、day-4 编号映射、修正政策、索引 |
| `agent_logs/operations/day-6/001..004` | 补 `目标` 与 `问题与修复` 段（内容基于当轮事实） |
| `agent_logs/operations/day-7/001..003` | 补 `目标` 与 `问题与修复` 段 |
| `agent_logs/operations/day-7/004` | 本记录 |

未删除任何历史记录。

## 测试 / 验收

本任务只改日志：

```text
字段扫描（UTF-8 解码）      -> day-6/day-7 记录均含目标/修改文件/命令/结果/测试/问题/Git
git diff --check            -> PASS
```

## Git

- commit：`docs(logs): normalize operation logs`（本记录所在提交）。
- push：随本轮各任务提交推送。

## 下一步

Task 5：Git 整洁检查（ignore、历史密钥、safety 脚本）。
