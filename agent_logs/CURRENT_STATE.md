# GIM Current State

- **PROJECT:** GIM
- **V1 STATUS:** COMPLETED（文本 IM + 文件 + 好友闭环 + Admin + 审计；V1.1 完成补验证）
- **Current Round:** 2026-11-12 Day 8 — race / Compose / Kafka / 浏览器 E2E / 性能最小场景补验证：COMPLETED
- **Branch:** `main`
- **Tags:** `gim-v1`（2026-10-15 基线，未移动）、`gim-v1.1`（本轮 annotated tag）
- **Working Tree:** 本日志提交前 clean
- **Next Authorized Work:** V2 或补充完整性能阶梯/soak（待用户决定）

## 当前验证状态（真实）

| 项目 | 状态 |
|---|---|
| 后端单测 `go test ./...` | PASS（69 包） |
| Linux Docker race `go test -race ./...` | **PASS**（exit 0，无 race warning） |
| 真实依赖集成（MySQL/Redis/etcd） | PASS（20 PASS / 1 SKIP；该 SKIP 的 Kafka 用例已补齐） |
| Kafka 真实 produce/consume/restart/幂等 | **PASS**（Compose broker） |
| Compose 全栈 build/up | **PASS**（17 services = 15 healthy + 2 exit 0） |
| 浏览器好友 E2E | **PASS**（Chrome，11 步） |
| 性能最小场景（HTTP profile c10 / Private 10 conn@10） | **PASS**（限两档，非容量结论） |
| Web / Admin 类型检查、测试、构建 | PASS（19 files 103 tests / 11 files 25 tests） |
| 仓库安全脚本 | PASS |

## 仍未验证

- 完整性能阶梯（25/50/100/200 连接、50 msg/s）、5 分钟 soak、restart 稳定性、文件吞吐档位。
- 远端 GitHub Actions 结果（`NOT OBSERVED`）。

## 范围与限制（不变）

- V1 仅文本消息；富消息、撤回、@、图文、好友上线提醒、`user_confs`、`DELETE /api/chat/chat`、delivery/read receipt、WS Ticket、多实例路由、文件 GC、审计 outbox/DLQ 均为 V2/后续。
- 好友申请仅接收方向；`isOnline` 非实时推送。
- Compose 单机，无 HA/内部 TLS/Secret Manager。

## Tag 说明

- `gim-v1`：2026-10-15 的 V1 基线，指向 `09994cb`，**未移动**。
- `gim-v1.1`：本轮最终验证状态的 annotated tag，指向 `55bbb8e`；包含项目整理、好友闭环、C1/C2 修复、文档/README/代码整洁，以及 race / Compose / Kafka / 浏览器 E2E 验证；性能仅最小两档，明确不声称完整阶梯或容量验证。

## Push 状态（已同步）

- `git push origin main`：成功，`bce78cc..76d2032 main -> main`。
- `git push origin gim-v1.1`：成功，`* [new tag] gim-v1.1 -> gim-v1.1`。
- 远端 `refs/heads/main`：先更新到 `76d2032`；状态记录提交 `da563b7` 推送后，`origin/main` 与本地 HEAD 一致（即本文件所在提交之后的远端状态）。
- 远端 tags：`gim-v1`（annotated `3db4382` → `09994cb`，未变化）、`gim-v1.1`（annotated `87949b5` → `55bbb8e`）。
- 未使用 `--force`；未移动/删除任何 tag。

## 环境恢复

- Compose 已 `down`（named volumes 保留）；Docker Desktop 已停止；临时 etcd 已停止；临时 E2E 脚本与含凭据的 host env 已删除。
