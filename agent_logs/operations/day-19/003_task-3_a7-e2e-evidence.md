# 003 / Task 3 — 澄清 A7 E2E 证据链

- **日期**：2026-11-14（Asia/Shanghai）
- **结果**：仓库仅 4 个通用 smoke；8 类专项 E2E 为临时脚本执行后删除。按“情况 B”如实标注，不补造脚本。

## 核实

仓库 `web/e2e/` 实际文件：

- `private-chat-smoke.mjs`
- `group-chat-smoke.mjs`
- `file-management-smoke.mjs`
- `admin-compose-smoke.mjs`

`admin/e2e/` 不存在；`scripts/` 只有 safety/race/perf 三个脚本。

操作日志确认（临时脚本删除）：

| 场景 | 证据 |
|---|---|
| 好友 E2E | `day-6/002_task-2_real-dependency-verification.md` |
| 图片 E2E | `day-10/004_task-3b_image-e2e-and-access-fix.md`（明确 `.tmp-image-e2e.mjs` 删除） |
| 群聊图片 E2E | `day-11/005_task-4_group-image-e2e.md`（临时脚本删除） |
| 文件 E2E | `day-12/005_task-3b_file-e2e.md` |
| 群聊文件 E2E | `day-13/002_task-2_group-file-e2e.md`（临时脚本删除） |
| 撤回 E2E | `day-14/003_task-2_frontend-recall.md` |
| 窗口 E2E | `day-15/002_task-1b_window-e2e.md` |
| 好友上线 E2E | `day-16/003_task-2_frontend-implementation.md` |

## 处理

- `docs/validation-status.md` 新增“E2E 证据链说明（2026-11-14 核实）”：明确 4 个 smoke 在库，8 类专项为临时脚本、未入库，证据为操作日志，复现需重写。
- `docs/known-issues.md` 新增 K-17，状态“部分 E2E 脚本未入库”。
- `docs/architecture-deep-dive.md` 9.1 A7 标记“已澄清”。
- 未新增/重写 E2E 脚本，未修改旧 PASS 结果。

## Git

- commit：`docs: clarify e2e evidence`（本记录所在提交）。
