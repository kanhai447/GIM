# 005 / Task 4 — 文档与验证状态

- **日期**：2026-11-14（Asia/Shanghai）
- **结果**：好友上线提醒从“设计”改为“已实现（单机）”；多实例限制与 best-effort 边界写入 docs/known-issues。

## 修改文件

| 文件 | 变更 |
|---|---|
| `docs/06_WEBSOCKET_PROTOCOL.md` | §5.7 改为已实现：payload、单机触发、过滤、多端、多实例 V2、队列/发送路径、best-effort；§3/§9 状态 |
| `docs/05_HTTP_API.md` | 好友 `isOnline` HTTP 权威 + WS 实时补充；已实现 |
| `docs/07_CORE_FLOWS.md` | §1.1 改为已实现，补充有界队列/Hub.SendToUser |
| `docs/08_REDIS_CONFIG.md` | Presence key/TTL 不变；好友通知只读 |
| `docs/known-issues.md` | V-19 移入已验证；V-20 多实例 NOT RUN；K-15 单机限制；K-16 offline best-effort |
| `docs/validation-status.md` | 好友提醒单测/集成/Web/E2E/race 真实结果 + 多实例 NOT RUN |
| `README.md` | V2 进度、验证结果、已知限制 |
| `.env.example`、`.env.compose.example` | 可选 friend notify 配置注释 |

## 事实

- 单机实现：Chat Hub 本地 0→1/1→0，有界队列 256/1/2s（可配），跳过自己/已删除/禁用，Hub.SendToUser。
- 多实例未实现，`GlobalPresenceCheck` 占位；文档明确 V2。
- 无 schema/Redis key 变更；Group WS 未改。
