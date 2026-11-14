# GIM Current State

- **PROJECT:** GIM
- **V1 STATUS:** COMPLETED（文本 IM + 文件 + 好友闭环 + Admin + 审计）
- **V2 STATUS:** Phase 1 完成（图片 `type=2` + 普通文件 `type=3` + 撤回 `type=4`）；好友上线提醒协议已定义、实现待下一轮
- **Current Round:** 2026-11-14 Day 15 — 撤回收尾 + 好友上线提醒协议设计 + `gim-v1.3`：COMPLETED
- **Branch:** `main`
- **Tags:** `gim-v1`（09994cb）、`gim-v1.1`（55bbb8e）、`gim-v1.2`（c97618f）、`gim-v1.3`（32a4a55）；前三个未移动
- **Next Authorized Work:** 好友上线提醒实现 / `user_confs` / 性能 / 是否准备 `gim-v2.0`

## 当前事实

- 撤回消息 `msg.type=4` 已实现：`withdrawMsg.msgID`、`RECALL_WINDOW_SECONDS`（默认 120）、Chat `2218–2222` / Group `2419–2423`、历史占位、文件下载撤销。
- 撤回收尾：并发测试（本次观察 1 条，K-14 记录边界）、短窗口 E2E（3s 窗口，4s 后 `2220`）、全量 race 均 PASS。
- 好友上线提醒：**仅协议**（docs/06 §5.7）——`friend.online`/`friend.offline`，Chat WS、仅通知好友、全局 0→1/1→0（多实例结合 Redis Presence）、Presence 权威、best-effort；实现 NOT RUN。
- 无需改 Chat Hub 并发模型：现有 `LocalConnectionTransitionObserver` 已满足；实现需组合 observer + 有界 worker。
- 无 schema/索引变更；管理员撤回未实现。

## 验证矩阵（真实执行）

| 项目 | 结果 |
|---|---|
| 后端 `go build` / `go vet` / `gofmt` / `go test ./...` | PASS / PASS / 无输出 / 69 包 PASS |
| 并发撤回测试 | **PASS**（观察 1 条；历史占位唯一；前端去重） |
| 撤回短窗口 E2E（3s） | **PASS**（3s 内成功；4s 后 `2220`；B 占位；恢复 120s） |
| 撤回代码全量 race | **PASS**（exit 0，无 DATA RACE） |
| Web `vue-tsc` / `vitest` | PASS / 21 files 132 tests |
| 好友上线提醒实现 | **NOT RUN**（协议设计完成） |

## 未验证（NOT RUN）

- 好友上线提醒后端/前端/推送（协议已定义）。
- 图片/文件性能、完整性能阶梯、soak/restart、远端 CI。
- K-14 严格单条需 schema 决策；管理员撤回未设计。

## Tag / Push

- `gim-v1.3` annotated tag：object `a5ed44f`，target `32a4a55`；已推送。
- 旧 tag peeled commit 未变：`09994cb` / `55bbb8e` / `c97618f`。
- 本轮 main `e3e37fd..32a4a55` 已推送；不 `--force`。
