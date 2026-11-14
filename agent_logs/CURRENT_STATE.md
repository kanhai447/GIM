# GIM Current State

- **PROJECT:** GIM
- **V1 STATUS:** COMPLETED（文本 IM + 文件 + 好友闭环 + Admin + 审计）
- **V2 STATUS:** 核心四 Phase 完成（图片 `type=2`、普通文件 `type=3`、撤回 `type=4`、好友上线提醒单机）；范围外项见 [docs/v2-scope.md](../docs/v2-scope.md)
- **Current Round:** 2026-11-14 Day 17 — V2 收口评估 + `gim-v2.0`：COMPLETED
- **Branch:** `main`
- **Tags:** `gim-v1`（09994cb）、`gim-v1.1`（55bbb8e）、`gim-v1.2`（c97618f）、`gim-v1.3`（32a4a55）、`gim-v1.4`（886d55e）、`gim-v2.0`（tag object `71032d5`，target `8e39668`）；旧 tag 未移动
- **Next Authorized Work:** 到此为止 / 补 user_confs / 管理员撤回 / 性能 / 多实例 Presence（均需另行授权）

## 当前事实

- V2 已实现并验证：图片、普通文件、撤回、好友上线提醒（单机）。
- 范围外（V2 后期/不做）：`user_confs`、管理员撤回、多实例 Presence 全局判断、好友申请实时推送、图片/文件吞吐与预览性能、语音/视频/@/图文、WS Ticket/Gateway 本地验签、Kafka outbox/多实例 WS 路由。
- 已知边界：K-14（并发撤回可能两条）、K-15（好友提醒单机）、K-16（offline best-effort）、E-8（授权查询无索引）、E-9（浏览器缓存不回收）。
- 验证：单测 70 包、集成（图片/文件/撤回/好友提醒）、前端 21 files/135 tests、全量 race、Compose、E2E 均 PASS；完整性能阶梯/soak/restart/远端 CI 为 NOT RUN/NOT OBSERVED。
- 无 schema 变更；Group WS 不修改 Presence。

## Tag / Push

- `gim-v2.0` annotated tag 指向包含 `docs/v2-scope.md` 的本轮收口 commit；已推送。
- 旧 tag peeled commit 未变：09994cb / 55bbb8e / c97618f / 32a4a55 / 886d55e。
- 不 `--force`，不移动/删除 tag。
