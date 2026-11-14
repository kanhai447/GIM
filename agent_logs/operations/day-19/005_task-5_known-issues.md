# 005 / Task 5 — 其余审计差异写入 known-issues

- **日期**：2026-11-14（Asia/Shanghai）
- **结果**：A1/A2/A3/A5/A6/A10 已写入 `docs/known-issues.md` 的“架构审计”小节；未改代码。

## 条目

| 编号 | 内容 | 状态 |
|---|---|---|
| A1 | deletion 两表生产只读、无写入方；隐藏走 hidden_at | 未使用表/待清理 |
| A2 | `DELETE /api/chat/chat` 无 HTTP 入口；HideSession 未接线 | 未实现 |
| A3 | 注册不初始化 user_confs；CanChat COALESCE | 已知边界（关联 K-9） |
| A5 | Group 消息仓库无 1213 重试 | 已知边界 |
| A6 | auth open_login 白名单无路由；settings open_login 存在 | 文档/实现不一致，已标注 |
| A10 | is_all_muted 只读、无写入入口 | 全员禁言未实现 |

## 说明

- 每条注明“发现于第十五轮深度文档审计”。
- 未删除既有 K/E/V 条目；A4 已在第十六轮修复，A7 已澄清，A8/A9/A11 已修文档，不重复列入。
- 与 v2-scope 的“V2 后期/不做”不重复展开，A3 引用 K-9。

## Git

- commit：`docs: record architecture audit findings`（本记录所在提交）。
