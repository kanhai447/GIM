# 002 / Task 2 — 修复 A4：Chat Admin 错误码与撤回重叠

- **日期**：2026-11-14（Asia/Shanghai）
- **结果**：Chat Admin 改为 `2230–2232`；Group Admin 保持 `2320–2322`；前端无按码映射；测试 PASS。

## 审计

| 模块 | 旧码 | 语义 | 冲突 |
|---|---|---|---|
| Chat Admin | `2220` | 私聊管理参数无效 | 与撤回“超过窗口”冲突 |
| Chat Admin | `2221` | 无管理员权限 | 与撤回“已撤回”冲突 |
| Chat Admin | `2222` | 消息不存在 | 与撤回“参数非法”冲突 |
| Chat 撤回 | `2218–2222` | 目标/本人/窗口/已撤回/参数 | — |
| Group Admin | `2320–2322` | 群管理参数/权限/不存在 | 与群撤回 `2419–2423` 不冲突 |
| User Admin | `1110–1112` | 用户管理 | 不冲突 |

- 前端 `web/src`、`admin/src` 未发现按 `2220–2222` 的分支映射；唯一 `2220` 出现在 `chat.test.ts`，是撤回测试。
- Chat Admin handler 测试只断言 HTTP 状态，不断言业务码。

## 修改

| 文件 | 变更 |
|---|---|
| `server/internal/chat/message/admin/transport/http/handler.go` | `codeInvalid 2220→2230`、`codeForbidden 2221→2231`、`codeNotFound 2222→2232`，加注释 |
| `docs/05_HTTP_API.md` | 新增 Chat Admin `2230–2232`、Group Admin `2320–2322` 行 |
| `docs/architecture-deep-dive.md` | 4.14.1 改用新码；9.1 A4 标记已修复 |
| `agent_logs/operations/day-19/002_task-2_a4-error-codes.md` | 本记录 |

未改撤回错误码，未改 Group Admin。

## 验证

```text
go build ./... / go vet ./... / gofmt -l .   PASS / PASS / 无输出
go test ./... -count=1                       PASS（70 packages）
web: vue-tsc PASS；vitest 21 files / 135 tests PASS
admin: vue-tsc PASS；vitest 11 files / 25 tests PASS
```

全仓 grep：Chat Admin 目录仅注释中出现 `2218-2222`，无旧码常量；`web/src/stores/chat.test.ts` 的 `2220` 为撤回语义。

## Git

- commit：`fix: separate admin error codes from recall`（本记录所在提交）。
