# 006 / Task 5 — 文档与验证状态（授权语义 + 收尾结果）

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = d699c87`（授权测试提交）。
- **结束状态**：本任务提交 `docs: record image phase authorization and validation`。

## 修改文件

| 文件 | 变更 |
|---|---|
| `docs/05_HTTP_API.md` | 新增“文件下载授权（V2 Phase 1）”小节：所有者、私聊发送者/接收方、群成员、退群/被踢、tombstone、非参与者 `2503`、文本不授权；JSON/LIMIT 1/无速率限制/客户端缓存边界 |
| `docs/06_WEBSOCKET_PROTOCOL.md` | 图片消息小节明确：消息 DTO 不含二进制，下载由文件服务按“所有者或消息参与者”授权 |
| `docs/07_CORE_FLOWS.md` | 图片流程补充下载授权检查步骤（tombstone/退群/非参与者 → 2503） |
| `docs/known-issues.md` | V-9（图片 race）、V-10（群聊图片 E2E）、V-11（授权专项测试）移入已验证；新增 V-12（图片性能）、K-12（撤回后授权待 V2）、E-8 更新（LIMIT 1/无索引/无速率限制）、E-9（浏览器缓存） |
| `docs/validation-status.md` | 新增“V2 Phase 1 收尾（2026-11-14）”：图片 race PASS、授权专项测试 PASS、群聊图片 E2E PASS；未验证项仅保留图片性能等 |
| `README.md` | 验证状态更新为 2026-11-14；图片 Phase 1 收尾结果；未验证项移除图片 race/群聊 E2E |
| `agent_logs/operations/day-11/004_task-3_race.md`、`005_task-4_group-image-e2e.md` | race 与群聊 E2E 证据 |

## 事实核对

- 授权专项测试覆盖 3.2 节全部语义：所有者、私聊接收方、私聊 tombstone、群成员、退群、非参与者、未知 fileID、文本不授权。
- 群聊图片 E2E 首次“退群后 200”经 SQL 直查确认是**浏览器缓存**，非服务端放行；`cache: 'no-store'` 复跑后 `2503`。已写入 docs/05 与 known-issues E-9。
- race 全量 exit 0，无 `WARNING: DATA RACE`。
- 无 schema/索引/migration 变更；未修改 Hub/广播/session/Presence/好友链路。
- 未打 tag；`gim-v1`/`gim-v1.1` 未移动。

## 测试

```text
go build ./... / go vet ./... / gofmt -l .        PASS / PASS / 无输出
go test ./... -count=1                            PASS（69 packages）
vue-tsc --noEmit                                  PASS
vitest run                                        PASS（21 files / 114 tests）
```

授权/集成测试与 race/E2E 结果见 002/003/004/005。

## Git

- commit：`docs: record image phase authorization and validation`（本记录所在提交）。
- 本轮不打 tag。
