# 001 / Task 1 — 文档一致性核对

- **日期**：2026-11-12（Asia/Shanghai）
- **开始状态**：`HEAD = origin/main = afdb552`，工作区干净；tag `gim-v1` 存在。
- **结束状态**：本任务提交 `docs: align contracts with implementation`。

## 目标

让 `docs/05`、`docs/04`、`docs/06`、`docs/01` 与当前实现逐条一致：补齐已实现但未记录的路由/索引/事件，并把未实现项明确标注为 V2，不删除原有条目。

## 1. HTTP 路由比对

方法：提取 `server/**/*.go`（排除 `_test.go`）中的 `rest.Route{Path: ...}` 字面量，补上配置驱动的 `/api/chat/ws/chat`、`/api/group/ws/chat` 与 `/healthz`；对 docs/05 提取 `/api/...` 路径并归一化 `:param`/`{param}` → `*`。

比对结果（修改前）：

```text
documented, not implemented:
  /api/{service}/            -> Gateway 路由模板，不是具体路由
  /api/auth/open_login       -> 已标注 V2
  /api/chat/admin/session    -> 已标注 V2
  /api/chat/chat             -> 已标注 V2
  /api/data/*                -> Mock-only，已注明无服务
implemented, not documented:
  /api/chat/ws/chat
  /healthz
```

修改：

| 文件 | 变更 |
|---|---|
| `docs/05_HTTP_API.md` | Chat 章节补 `GET /api/chat/ws/chat?token=`（Upgrade、Auth/Origin、`CHAT_WS_PATH` 可配置）；§1 补 `GET /healthz`（仅进程就绪 `{"status":"ok"}`，非业务 API） |

修改后：所有已实现 `/api` 路由均已在 docs/05 出现；未实现项仅剩上述 V2/Mock/模板条目，均有标注。

错误码：docs/05 §7 的 11xx 分配表（1110–1112 Admin、1113–1119 Friend）与代码一致；friend 包只使用 1113–1119，无重复。

## 2. 数据库比对

方法：从 `server/migrations/*.up.sql` 提取表、UNIQUE、普通索引与 CHECK；与 `docs/04_DATABASE_DESIGN.md` 比对名称级覆盖。

真实统计：

```text
tables = 17
UNIQUE = 16
plain indexes = 30
CHECK constraints = 40
```

发现：docs/04 原有 prose 覆盖了所有 17 张表，但**没有记录索引/约束标识符**；实现侧未发现缺表、缺索引或与文档冲突的约束。

修改：

| 文件 | 变更 |
|---|---|
| `docs/04_DATABASE_DESIGN.md` | 新增 §7.11「实际索引与约束清单」，逐表列出 migration、UNIQUE、普通索引与 CHECK 数，并注明 008 只加列、不新增索引 |

## 3. WebSocket 比对

代码实际事件（`chat/protocol`、`group/ws/protocol`）：

```text
chat.send / chat.ack / chat.message / error
group.send / group.ack / group.message / error
```

docs/06 覆盖以上事件；`friend.online` / `friend.offline` 原文写成可用事件，但服务端没有实现。

修改：

| 文件 | 变更 |
|---|---|
| `docs/06_WEBSOCKET_PROTOCOL.md` | Presence 段改为 **V2 / 未实现**（V1 只通过 HTTP 读取 `isOnline`）；新增 §9「V1 事件清单」表，标明已实现事件与 V2 事件 |

## 4. 架构与拓扑

- README 与 docs/01 无 `Group API → Group RPC` 残留；只剩“已删除”说明。
- `server/cmd/` 11 个入口与文档逐一核对，全部有对应说明。
- `docker compose config --services` = **17**；docs/01 §6 补充该数字与两个一次性 job 的说明。
- 修正 docs/01 §4 的服务职责：`open_login`、`user_confs`、富消息/撤回、群资料编辑/搜索、文件预览、日志删除均标注 V2 或删除；新增 §4.1 `cmd/` 进程入口表。

## 修改文件汇总

- `docs/01_PROJECT_ARCHITECTURE.md`
- `docs/04_DATABASE_DESIGN.md`
- `docs/05_HTTP_API.md`
- `docs/06_WEBSOCKET_PROTOCOL.md`

## 测试 / 验收

本任务只改文档：

```text
git diff --check                  -> PASS
go build ./...                    -> PASS（确认未误改代码）
docker compose config --quiet     -> PASS
```

## 问题与修复

- 路由比对脚本只提取 `/api/...`，`/healthz` 不在该前缀内，曾被误报为“未文档化”；实际已在 docs/05 §1 补充说明，比对结论中已注明。
- docs/04 的索引/约束此前只有 prose、没有标识符；本轮以 migration 为准补 §7.11，未发现实现缺表/缺索引。

## Git

- commit：`docs: align contracts with implementation`（本记录所在提交）。
- push：随本轮各任务提交推送。

## 下一步

Task 2：重写根 README 与三个子 README。
