# Day 15 — 撤回收尾 + 好友上线提醒协议 + gim-v1.3（2026-11-14）

## 今日目标

撤回收尾（并发测试、短窗口 E2E、全量 race）；好友上线提醒协议设计（不实现）；打 `gim-v1.3` 并 push。不打 `gim-v2.0`，不改 schema。

## 真实结果

| 项目 | 结果 | 证据 |
|---|---|---|
| 并发撤回测试 | **PASS**（观察 1 条；K-14 记录边界） | `TestConcurrentRecallSameTargetIsSafe`；前端重复撤回行合并测试 |
| 撤回短窗口 E2E | **PASS** | `RECALL_WINDOW_SECONDS=3`：3s 内成功；4s 后 `2220`、原消息保留；已恢复默认 120s |
| 撤回代码全量 race | **PASS** | Linux `golang:1.25-bookworm` `go test -race ./...`，exit 0，无 DATA RACE |
| 好友上线提醒协议 | **设计完成，实现 NOT RUN** | docs/06 §5.7 + docs/05/07/08 |
| 后端全量 | **PASS** | `go build` / `go vet` / `gofmt` / `go test ./...`（69 packages） |
| Web | **PASS** | `vue-tsc`；`vitest` 21 files / **132 tests** |

## 关键决策

- K-14 不强行断言“只有一条”；无唯一索引时并发可能两条，历史占位唯一、前端合并去重、授权不受影响；严格一条需 schema 变更，本轮不做。
- 好友上线提醒：Chat WS 事件、仅通知好友、全局 0→1/1→0（多实例结合 Redis Presence）、Presence 权威、best-effort；现有 Hub observer 足够，无需改并发模型。
- 短窗口 E2E 使用本地 `.env.compose.local` 临时配置，完成后恢复并重启验证默认 120s。

## Commit / Tag

- `cb0556c` test(chat): cover concurrent recall boundary
- `32a4a55` docs: define friend online reminder protocol
- annotated tag `gim-v1.3`（tag object `a5ed44f`，target `32a4a55`）
- main push `e3e37fd..32a4a55`；tag push new `gim-v1.3`
- 旧 tag peeled commit 未变：`09994cb` / `55bbb8e` / `c97618f`

## 未验证 / 剩余

- 好友上线提醒实现（observer/worker/推送、前端 isOnline 更新）NOT RUN。
- 图片/文件性能、完整性能阶梯、soak/restart、远端 CI 仍未跑。
- K-14 严格单条需 schema 决策；管理员撤回未设计。

## 下一步

1. 好友上线提醒实现（协议已就绪）。
2. `user_confs` / 性能。
3. 是否准备 `gim-v2.0`（需另行授权）。
