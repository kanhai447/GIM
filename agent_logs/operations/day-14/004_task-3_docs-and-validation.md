# 004 / Task 3 — 文档与验证状态（撤回）

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = dc63ab7`（前端撤回）。
- **结束状态**：本任务提交 `docs: record recall support and validation`。

## 修改文件

| 文件 | 变更 |
|---|---|
| `docs/06_WEBSOCKET_PROTOCOL.md` | §5.6 从“设计”改为“已实现”；补充 `2222/2423` 参数错误、历史占位/前端合并、群成员最后校验、文件授权 `NOT EXISTS`、并发重复撤回边界 |
| `docs/05_HTTP_API.md` | 撤回错误码表确认并新增 `2222/2423`；下载授权“撤回后非所有者失效”改为已实现表述 |
| `docs/07_CORE_FLOWS.md` | §2.3 撤回流程改为“已实现” |
| `docs/04_DATABASE_DESIGN.md` | 撤回复用 msg JSON、无新列、`[撤回]` preview 改为已实现 |
| `docs/known-issues.md` | V-16 撤回实现移入已验证；V-17 窗口 E2E NOT RUN；K-1/K-12 更新；新增 K-14 并发重复撤回边界；E-8 更新 NOT EXISTS |
| `docs/validation-status.md` | 新增“撤回实现”验证表（单测/集成/授权/Web/E2E + 窗口 E2E NOT RUN） |
| `README.md` | V2 Phase 1 增加撤回；未实现列表移除撤回、补充管理员撤回；验证状态加入撤回结果；限制说明更新 |

## 事实核对

- 撤回实现无 schema/索引/migration 变更：复用 `msg` JSON，原消息行保留。
- 校验顺序与 §5.6 一致：目标存在/会话 → 本人 → tombstone → 已撤回 → 窗口 → 群成员；参数缺失 `2222/2423`。
- 文件授权：`NOT EXISTS` 排除被撤回原消息，保持 `SELECT 1 … LIMIT 1`；所有者不受影响。
- 群聊发送者已退群时返回 `2404`（按协议“仍为成员”），未另行询问。
- 撤回窗口浏览器 E2E 未做（默认 120s）；单测/集成覆盖 `2220`。
- 未打 tag；三个 tag 未移动。

## 测试

```text
go build ./... / go vet ./... / gofmt -l .        PASS / PASS / 无输出
go test ./... -count=1                            PASS（69 packages）
vue-tsc --noEmit                                  PASS
vitest run                                        PASS（21 files / 131 tests）
```

撤回集成/E2E 证据见 001/002/003 与 validation-status。

## Git

- commit：`docs: record recall support and validation`（本记录所在提交）。
