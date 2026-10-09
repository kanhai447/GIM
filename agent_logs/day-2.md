# Day 2

## 授权状态

- Day 2：AUTHORIZED。
- Phase 2/3 业务：NOT STARTED。
- 已完成：Pre-Day2 Concurrency Preflight。

## Pre-Day2 Concurrency Preflight

- 时间：2026-10-09 10:43 +08:00。
- 范围：仅检查 race 环境、Chat schema、Redis、JWT/WS handshake、Gateway Upgrade 和 FIM WebSocket 参考问题。
- 业务代码：无修改；未建立 WebSocket、Hub、Client、readPump、writePump 或 Presence。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT。Go 1.25.2 / windows-amd64 / CGO=1；当前 MinGW.org GCC 6.3.0 为 `mingw32/i586`，`go test -race ./...` 报 `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`。Day 2 后续必须在兼容环境/CI 验证，不阻塞开发。
- Chat Schema：READY；幂等 unique、session unique、last message/read、unread、top、用户维度隐藏/删除字段全部符合文档。
- Redis：Client READY；Presence/可选 connection tracking helper 待下一 Checkpoint 在 `rediskeys` 集中新增。
- Auth/JWT：READY；WS query token 由 Gateway 调 Auth 验证，继续 V1 `?token=JWT`，不引入 ticket 或本地重复验签。
- Gateway：READY；显式 Chat route、Upgrade detection、Upgrade 无普通 proxy timeout 和标准 ReverseProxy 条件齐备，focused tests PASS。
- Reference：确认全局普通 map、直接并发 Conn write、无 writePump/Ping/Pong、CheckOrigin 过宽和多设备 Presence/投递语义问题；未复制代码。
- 非 race 回归：`go test ./... -count=1` PASS。
- Day 2 blocker：NONE。

## Operation Log

- `agent_logs/operations/day-2/001_pre-day2_concurrency-preflight.md`

## Next Checkpoint

Chat WebSocket Hub / Client foundation
