# 004 / Task 4 — 遗留清理与错误码核对

- **日期**：2026-11-11（Asia/Shanghai）
- **开始状态**：`HEAD = d3be2c9`，working tree clean。
- **结束状态**：本任务提交 `chore: drop unused rpc env keys and document friend error codes`。

## 目标

清理无用 RPC 环境键、核查 group api 残留、确认 1113–1119 错误码已文档化且无冲突；删除仅限有 grep 证据的对象。

## 5.1 清理未使用的 RPC 环境键

grep 证据（清理前）：

```text
server/**/*.go            -> 0 处引用 AUTH_RPC_PORT / CHAT_RPC_PORT / FILE_RPC_PORT / SETTINGS_RPC_PORT
.env.example:53-57        -> AUTH_RPC_PORT / USER_RPC_PORT / CHAT_RPC_PORT / FILE_RPC_PORT / SETTINGS_RPC_PORT
.env.compose.example:46,62 -> USER_RPC_HOST / USER_RPC_PORT
```

变更：

```diff
-.env.example
-AUTH_RPC_PORT=CHANGE_ME
 USER_RPC_PORT=CHANGE_ME
-CHAT_RPC_PORT=CHANGE_ME
-FILE_RPC_PORT=CHANGE_ME
-SETTINGS_RPC_PORT=CHANGE_ME
```

- 保留 `USER_RPC_HOST` / `USER_RPC_PORT`（Auth/Chat/Group 实际通过 `RPCAddress(values,"USER")` 读取）。
- 未改 `.env.local`（ignored，本机运行配置）。

## 5.2 `server/api/group/v1` 残留核查

```text
Test-Path server/api/group/v1                 -> False（目录已在 round 2 删除）
server/**/*.go grep "api/group/v1"            -> 0 处
server/api 目录现存                            -> 仅 server/api/user/v1/{user.pb.go,user.proto,user_grpc.pb.go}
非 Go 文件 grep group-rpc/GROUP_RPC/GroupServiceClient（排除历史 agent_logs）
  -> 仅剩 3 处“已删除”说明文本：docs/01、docs/05、README
```

结论：无孤儿代码、无 import 方，`api/user/v1` 保留（Auth/Chat/Group 仍依赖 User RPC）。

## 5.3 错误码核对

代码事实：

```text
internal/user/friend/friend.go      1113..1119（7 个，唯一使用者）
internal/user/admin/transport/http  1110..1112（Admin 用户管理）
internal/user/service               1001/1101/1102
```

- 1113–1119 与既有码**无重复**，且只被 friend 包使用。
- docs/05 §7 原先只列 1001/1101/1102，未列 1110–1119；本轮新增 11xx 分配表，逐码写明归属与语义，并注明“新增码必须登记”。
- 未修改任何既有错误码。

## 5.4 验证

```powershell
go build ./...                 -> PASS
go vet ./...                   -> PASS
go test ./... -count=1         -> PASS（69 个包 ok）
docker compose config --quiet  -> PASS
docker compose config --services -> 17 个 service
.env.example grep 'RPC'        -> 仅 USER_RPC_PORT
```

**计数修正**：`docker compose config --services` 实际为 **17**（删除 group-rpc 前为 18）。round-2 的 day-5 日志/汇总写成“19 → 18”属误计，本轮已在对应历史记录中加注修正，不改变任何代码或 compose 内容。

## 问题与修复

- 发现 Compose service 数历史误计（19/18），实际 18→17；已在 day-5 记录与汇总中加注修正，不改变 compose 内容。
- `server/api/group/v1` 已不存在且无 import，无需删除；`server/api` 仅剩 user 契约。
- 错误码无冲突：1113–1119 只被 friend 包使用，1110–1112 为 Admin 用户管理。

## Git

- commit：`chore: drop unused rpc env keys and document friend error codes`（本记录所在提交）。
- push：随本轮最终汇总提交一起推送。

## 下一步

全量验收（build/vet/test/前端/compose/safety）并更新 `agent_logs/CURRENT_STATE.md` 与 `day-6.md`。
