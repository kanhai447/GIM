# 002 / Task 2 — 对齐 V2 范围引用

- **日期**：2026-11-14（Asia/Shanghai）
- **结果**：现有文档统一引用 `docs/v2-scope.md`；未改协议内容，未新增未验证声明。

## 修改清单

| 文件 | 变更 |
|---|---|
| `README.md` | 新增“V2 范围”小节链接 v2-scope；tag 列表加入 `gim-v2.0` |
| `docs/validation-status.md` | 新增“V2 收口（2026-11-14）”小节；保留全部 NOT RUN/NOT OBSERVED |
| `docs/known-issues.md` | 顶部增加 v2-scope 引用；未删除条目 |
| `docs/04_DATABASE_DESIGN.md` | 顶部一行 v2-scope 引用 |
| `docs/05_HTTP_API.md` | 顶部一行 v2-scope 引用 |
| `docs/06_WEBSOCKET_PROTOCOL.md` | 顶部一行 v2-scope 引用 |
| `docs/07_CORE_FLOWS.md` | 顶部一行 v2-scope 引用 |
| `agent_logs/CURRENT_STATE.md` | V2 核心四 Phase 完成 + v2-scope 链接 + v2.0 tag |
| `server/README.md` | 旧“富消息/V2”表述改为已实现 + v2-scope 链接 |
| `web/README.md` | “仅文本/V2”改为已支持 + v2-scope 链接 |

## 一致性检查

- `v2-scope.md` 的 K-14/K-15/K-16/E-8/E-9 与 known-issues 一致。
- `v2-scope.md` 的 PASS/NOT RUN 与 validation-status 一致；未删除 NOT RUN。
- 未发现“全量验证通过”“支持百万连接”等未验证声明（`docs/10_TEST_ACCEPTANCE.md` 仅作为写作规范提及）。
- README/子 README 不再声称“好友上线提醒为 V2 待做”。
- 无业务代码变更。

## Git

- commit：`docs: align V2 scope references`（本记录所在提交）。
