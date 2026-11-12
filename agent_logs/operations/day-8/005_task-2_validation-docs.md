# 005 / Task 2 — 更新验证状态与已知问题

- **日期**：2026-11-12（Asia/Shanghai）
- **开始状态**：`HEAD = bce78cc`；本轮补验证已完成（见 001–004）。
- **结束状态**：本任务提交 `docs: update validation status after round-5 checks`。

## 修改文件

| 文件 | 变更 |
|---|---|
| `docs/validation-status.md` | 新增 2026-11-12 验证矩阵：race PASS、Compose PASS、Kafka PASS、浏览器 E2E PASS、性能最小两档 PASS（含命令、环境、数字）；未验证项收窄为完整性能阶梯/soak/restart/文件吞吐与远端 CI |
| `docs/known-issues.md` | V-1（race）、V-2（Compose）、V-3（Kafka）、V-5（浏览器 E2E）移入“已验证”；保留 V-4（完整性能/soak/文件吞吐）与 V-6（远端 CI）；V2 与工程限制不变 |
| `README.md` | 验证状态更新为 2026-11-12 结果；tag 说明更新：`gim-v1` 基线未移动，`gim-v1.1` 包含 race/Compose/Kafka/E2E，明确不含完整性能阶梯/soak/容量验证 |
| `agent_logs/day-8.md`、`agent_logs/CURRENT_STATE.md` | 本轮汇总与当前状态（只写当前，不写历史流水） |
| `agent_logs/prompts/round-5.md` | 本轮指令全文落盘 |

## 事实核对

- 集成测试的 1 个 SKIP（Kafka）已由本轮的 Compose broker 真实用例补齐；文档注明两个日期的证据来源。
- 性能只复跑两个最小档位，文档明确“不是完整基线、不是容量承诺”。
- 未修改 `docs/00_DECISIONS.md`，未发现需要修改决策表的冲突。
- 未修改业务代码。

## 测试 / 验收

本任务只改文档：

```text
git diff --check -> PASS
```

代码测试已在 001–004 覆盖。

## Git

- commit：`docs: update validation status after round-5 checks`（本记录所在提交）。
- push：提交后推送 `origin/main`；tag 见 006。

## 下一步

Task 3：创建并推送 `gim-v1.1` annotated tag。
