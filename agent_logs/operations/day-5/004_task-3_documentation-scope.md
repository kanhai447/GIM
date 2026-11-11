# 004 / Task 3 — 文档口径收缩

- **日期**：2026-11-11（Asia/Shanghai）
- **开始状态**：HEAD `f0d3b6d`，working tree clean。
- **结束状态**：HEAD `83db97a`（本日志所在提交另计）。

## 目标

让 AGENTS.md、README、三个子 README 与 docs/05/06/07/10 的表述与当前实现一致：明确 V1 已实现范围、V2 未实现项、历史数据的时效性，不删除原始目标清单。

## 修改文件

| 文件 | 变更 |
|---|---|
| `AGENTS.md` | §5 标题改为“原始目标清单（保留为 V2 目标）”，新增 §5.1 范围分层：V1 已实现 / V2 计划 |
| `README.md` | 完成声明改为“文本 IM + 文件 + 好友闭环 + Admin + 审计”；Features 增加好友闭环、删除撤回/用户配置的未实现声明；Reliability/Security 补充好友事务与 Presence 只读；测试计数 19/103；Performance 标注历史数据未重新采集；Known Limitations 更新好友/环境限制；Roadmap 移除已完成项；拓扑注明 Group RPC 无调用方、任务四移除 |
| `server/README.md` | 重写命令边界（去掉不存在的 Auth/Chat/File/Settings RPC）；删除 Checkpoint 旧状态；启动列表补 `settings-api`、`migrate`，删 `group-rpc`；新增 Friend API 与 WebSocket 小节 |
| `admin/README.md` | Dashboard 改为真实 `/api/settings/admin/overview`，说明不使用 `/api/data/*` Mock |
| `docs/05_HTTP_API.md` | User 章节改为已实现/V2 分层；补 8 条好友路由、字段、错误码、Presence 语义；`open_login`、`chat/admin/session`、`DELETE /api/chat/chat`、`PUT user_info`、`POST user/valid` 标注 V2；文件下载路径统一为 `:id` |
| `docs/06_WEBSOCKET_PROTOCOL.md` | 消息类型明确“V1 只实现文本 type=1”，其余 V2；撤回标注 V2/未实现 |
| `docs/07_CORE_FLOWS.md` | 好友流程改为实际实现：事务、行锁、pending 唯一键、并发同意语义、Presence 只读 |
| `docs/10_TEST_ACCEPTANCE.md` | 新增 2A 好友闭环验收场景 |
| `docs/interview/project-overview.md` | 亮点补好友并发；Honest gaps 更新为 V2 项 |
| `docs/interview/resume-bullets.md` | 测试计数更新为 19/103，补好友闭环 bullet |
| `docs/performance/baseline.md` | 历史 Web 测试计数标注时点与后续值 |

## 验收：docs/05 路由与代码逐条比对

用脚本从 `docs/05_HTTP_API.md` 提取 `/api/...` 路径（归一化 `:id`/`{id}` → `*`），与 `server/**/*.go`（排除 `_test.go`）中 `Path: "..."` 注册的路由比对。

结果只剩预期差异：

```text
documented but NOT implemented:
  /api/auth/open_login          -> 已标注 V2 / 未实现
  /api/chat/admin/session       -> 已标注 V2 / 未实现
  /api/chat/chat                -> 已标注 V2 / 未实现
  /api/data/*                   -> 文档已注明 Mock-only，无服务
  /api/{service}/               -> Gateway 路由模板，不是具体路由

implemented but NOT documented:
  /healthz                      -> 基础设施探针
  /api/file/*                   -> 归一化后与文档 :id 等价（已统一写法）
  /api/group/ws/chat            -> 由 GROUP_WS_PATH 配置注册，文档 06 已描述
```

## 执行命令与结果

```powershell
git diff --check                              -> PASS
路由比对脚本（见上）                            -> 仅剩 V2/Mock/模板/健康探针差异
```

本任务只改文档，未运行代码测试；代码测试在 Task 2/4 覆盖。

## Git

- commit：`83db97a docs: align the v1 scope statement with the implementation`
- push：随本日志一同推送。
- tag：无。

## 遗留

- README 拓扑中的 `Group RPC` 将在 Task 4 删除服务后移除。
- `agent_logs/CURRENT_STATE.md` 仍是 “V1 frozen” 状态，将在本轮收尾统一更新。

## 下一步

Task 4：删除无调用方的 group-rpc。
