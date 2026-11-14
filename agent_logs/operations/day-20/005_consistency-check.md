# 005 / Task 4 — 面试问答一致性核对

- **日期**：2026-11-14（Asia/Shanghai）
- **结果**：问答与 `v2-scope.md`、`validation-status.md` 一致；发现 known-issues 的 K-3 为历史条目，与第十三轮好友上线实现不一致，本轮不修改 known-issues，按要求记录。

## 核对

| 来源 | 结论 |
|---|---|
| `docs/v2-scope.md` | 四 Phase（图片/文件/撤回/好友上线单机）与问答第 1/2.9 节一致；范围外项一致 |
| `docs/validation-status.md` | 性能只写 HTTP c10、Private 10 conn 两档；E2E 明确 4 个 smoke + 临时脚本未入库（K-17）；NOT RUN 项未夸大为 PASS |
| `docs/known-issues.md` | K-14/K-15/K-16/K-17、A1/A2/A3/A5/A6/A10、E-8/E-9 均按原文引用；未编造编号 |
| 深度文档 | 第 4 章 16 个小节与问答 3.1–3.16 一一对应；未发现深度文档与代码冲突 |

## 已知不一致（只记录，不改）

- known-issues `K-3` 仍写“好友 isOnline 非实时，无 friend.online/offline 推送”，但第十三轮已实现单机好友上线提醒；当前有效边界是 K-15（单机）/K-16（offline best-effort）。问答采用 K-15/K-16，未引用 K-3 作为“无推送”依据。建议后续修订 K-3 措辞。

## 引用抽查（10 个，全部存在）

`server/internal/chat/friendnotify/notifier.go`、`server/internal/chat/presence/service.go`、`server/internal/file/repository/mysql/repository.go`、`server/internal/chat/message/repository/mysql/repository.go`、`server/internal/group/message/repository/mysql/repository.go`、`server/internal/logs/consumer/consumer.go`、`server/internal/platform/audit/metadata.go`、`server/migrations/runner.go`、`server/internal/chat/hub.go`、`server/internal/platform/kafkaaudit/producer.go`。

## Git

- commit：`docs: record interview consistency check`（本记录所在提交）。
