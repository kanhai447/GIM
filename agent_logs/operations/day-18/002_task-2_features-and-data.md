# 002 / Task 2 — 架构文档：群聊、Admin、审计、Migration

- **日期**：2026-11-14（Asia/Shanghai）
- **结果**：完成 `docs/architecture-deep-dive.md` 第 4 章剩余功能（群聊/Admin/审计/migration）与第 5–10 章；未改业务代码。
- **覆盖**：
  - 4.11 群创建/成员/角色/邀请/申请/禁言
  - 4.12 群文本/图片/文件消息
  - 4.13 群撤回
  - 4.14 Admin 私聊审计/群管理/文件管理/设置/日志查询
  - 4.15 审计 LogEvent/Kafka producer/consumer 幂等
  - 4.16 migration runner/dirty 保护
  - 第 5 章 18 张表 + docs/04 差异 D1–D9
  - 第 6 章并发模型（已在前一提交，本章校验）
  - 第 7 章配置键/compose/端口/job
  - 第 8 章测试地图 + validation-status 出入
  - 第 9 章边界 + 文档实现差异 A1–A11
  - 第 10 章文件索引

## 关键差异（只记录）

- `chat/message/admin` 错误码 `2220–2222` 与撤回错误码数值重叠。
- Group 消息仓库无 1213 重试。
- `chat_message_deletions` / `group_message_deletions` 无生产写入方。
- `DELETE /api/chat/chat` 未找到 HTTP 入口。
- docs/04 的列名、`user_confs` 初始化、`friends` 草案字段与 migration 存在差异。
- validation-status 的多个浏览器 E2E 无入库脚本（仓库仅 4 个 smoke）。

## Git

- commit：`docs: document group, data and deployment architecture`（第 4 章剩余 + 5–10 章；如分批提交，本记录覆盖对应内容）。
