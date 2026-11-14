# Day 12 — gim-v1.2 tag + 普通文件消息 Phase 1（2026-11-14）

## 今日目标

打 `gim-v1.2` tag（图片 Phase 1 收尾）；实现普通文件消息 `msg.type=3` 完整闭环；不打 `gim-v2.0`。

## 真实结果

| 项目 | 结果 | 证据 |
|---|---|---|
| `gim-v1.2` annotated tag | **创建并推送成功** | 指向 `c97618f`；远端 `a2f5885 → c97618f`；`gim-v1`/`gim-v1.1` 未移动 |
| 文件消息协议 | 完成 | `docs/06` §5.5、`docs/05` 上传/错误码/授权 |
| 后端构建/单测 | **PASS** | `go build` / `go vet` / `gofmt` / `go test ./...`（69 packages） |
| 聊天文件集成（真实 MySQL/Redis/WS） | **PASS** | ACK、接收、历史、重复 clientMsgId 仅 1 行、`2215/2216` 不落库、接收方可解析文件 |
| 群聊文件消息 | PASS（单测） | 归一化、幂等冲突、`2416–2418`；授权测试含群文件消息 |
| 文件下载授权（文件消息） | **PASS** | 群成员可下载；tombstone/退群/非成员 `2503`；owner 仍可下载 |
| Web 类型/测试/构建 | **PASS** | `vue-tsc`；`vitest` 21 files / 122 tests；`vite build` |
| 浏览器文件 E2E | **PASS** | A 发 `report.pdf` → B 收到卡片并下载 → 非参与者 D `2503` |
| 图片/文件消息吞吐性能 | **NOT RUN** | 本轮未做 |

## 关键过程

1. 先确认 `HEAD = origin/main = c97618f`，创建并推送 `gim-v1.2`，校验远端三个 tag 与前两个 hash 不变。
2. 协议先行：`type=3` + `fileMsg`；不限制 MIME 前缀；`FILE_MESSAGE_MAX_BYTES` 默认 25 MiB；Chat `2215–2217`、Group `2416–2418`。
3. 后端复用图片路径：`GetOwnedFile` 查询 `user_files.original_name`/`file_objects`；服务端回填元数据；同一事务/ACK/幂等。
4. 授权查询从仅 `imageMsg.fileID` 扩展为同时匹配 `fileMsg.fileID`，保持 `LIMIT 1` 和 tombstone/成员过滤。
5. 前端新增“文件”入口、`sendFile`、文件卡片（文件名/大小/鉴权下载/失败重试），私聊/群聊共用。
6. Compose 全栈重建并跑文件 E2E，一次 PASS；清理环境。

## 修改文件

- 协议/文档：`docs/06`、`docs/05`、`docs/07`、`docs/04`、`README.md`、`docs/validation-status.md`、`docs/known-issues.md`。
- 后端：两个 protocol、两个 message service/repository（+mysql）、两个 config、`cmd/chat-api`、`cmd/group-api`、file 授权 SQL、env 模板、测试。
- 前端：类型、utils、两个 socket、两个 store、`MessageBubble`、两个 Composer、两个 view、测试。
- 日志：`agent_logs/operations/day-12/001–006`。

## 问题与修复

- 无阻塞问题；文件消息授权需同时匹配 `fileMsg.fileID`，已随后端实现修复并加入测试。

## 当日验收

- 文件消息闭环完成：上传 → 发送 → 事务落库 → ACK → 推送 → 历史 → 渲染 → 鉴权下载。
- 无 schema/索引变更；未打 `gim-v2.0`；三个已有 tag 均未移动。

## 下一步

- 可选：撤回 / 好友上线提醒 / `user_confs` / 图片与文件消息性能；是否打 `gim-v2.0` 待 Phase 整体验收后决定。
