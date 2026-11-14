# GIM Current State

- **PROJECT:** GIM
- **V1 STATUS:** COMPLETED（文本 IM + 文件 + 好友闭环 + Admin + 审计）
- **V2 STATUS:** Phase 1 完成（图片 `type=2` + 普通文件 `type=3` + 撤回 `type=4`）；语音/视频/回复/引用/@/图文未实现
- **Current Round:** 2026-11-14 Day 14 — 撤回实现（后端 + 前端 + 测试 + 文档）：COMPLETED
- **Branch:** `main`
- **Tags:** `gim-v1`（09994cb）、`gim-v1.1`（55bbb8e）、`gim-v1.2`（c97618f）；均未移动，本轮未打新 tag
- **Next Authorized Work:** 好友上线提醒 / `user_confs` / 图片与文件性能 / 是否打 `gim-v2.0`

## 当前事实

- 撤回消息 `msg.type=4`：`withdrawMsg.msgID` 指向原消息；沿用 `chat.send`/`group.send`；服务端按“目标存在/会话 → 本人 → 未 tombstone → 未撤回 → `RECALL_WINDOW_SECONDS`（默认 120）→ 群成员”校验。
- 错误码：Chat `2218–2222`、Group `2419–2423`；群非成员沿用 `2404`。
- 幂等：同 sender+target 顺序重复撤回返回同一 ACK；同 clientMsgId 幂等不变；并发不同 clientMsgId 可能重复（K-14，无唯一索引）。
- 历史/session：被撤回原消息替换为 `type=4` 占位、`[撤回]`，不返回原 payload；撤回消息自身可见；Web 在目标可见时合并为一条占位。
- 文件下载授权：撤回后非所有者下载返回 `2503`；所有者仍可下载；授权查询 `NOT EXISTS` + `LIMIT 1`，无 schema/索引变更。
- Web：自己发送的文本/图片/文件消息在 120s 内显示“撤回”；成功 ACK/推送后本地替换占位；失败显示错误、原消息保留。
- 管理员撤回未实现（K-13 / V2）。

## 验证矩阵（真实执行）

| 项目 | 结果 |
|---|---|
| 后端 `go build` / `go vet` / `gofmt` / `go test ./...` | PASS / PASS / 无输出 / 69 包 PASS |
| 撤回集成（真实 MySQL/Redis/WS） | **PASS**（ACK、接收、历史占位、重复撤回、窗口/非本人/tombstone、文件下载撤销） |
| 文件授权联动 | **PASS**（接收方 `ErrNotFound`，owner 200） |
| Web `vue-tsc` / `vitest` / `vite build` | PASS / 21 files 131 tests / PASS |
| 浏览器撤回 E2E | **PASS**（文本+文件撤回、占位、B 下载 2503、owner 200） |

## 未验证（NOT RUN）

- 撤回窗口浏览器 E2E（默认 120s；单测/集成覆盖 `2220`）。
- 撤回代码单独 race（本轮未要求）。
- 图片/文件消息吞吐与预览性能。
- 完整性能阶梯、soak/restart、文件吞吐、远端 CI（历史遗留）。

## Tag / Push

- `gim-v1`/`gim-v1.1`/`gim-v1.2` 均未移动；本轮未打新 tag。
- 本轮后端/授权/前端/文档 commit 推送至 `origin/main`；不 `--force`。
