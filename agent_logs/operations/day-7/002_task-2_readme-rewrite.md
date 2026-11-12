# 002 / Task 2 — README 重写

- **日期**：2026-11-12（Asia/Shanghai）
- **开始状态**：`HEAD = c180523`。
- **结束状态**：本任务提交 `docs: rewrite readmes to current state`。

## 目标

把根 README 与三个子 README 从历史 checkpoint 叙述改写为当前事实：V1/V2 范围、17 service 拓扑、真实启动/测试命令、验证状态与已知限制；同时新增 `docs/validation-status.md` 与 `docs/known-issues.md` 作为后续引用入口。

## 修改文件

| 文件 | 变更 |
|---|---|
| `README.md` | 全量重写：定位、V1 已实现/V2 清单、无 group-rpc 的架构图、技术栈、目录、环境要求、Compose 启动、本地原生启动（migration + 9 个服务顺序）、前端启动、默认配置、测试命令、验证状态、tag 说明（`gim-v1` 基线 / `gim-v1.1` 第五轮）、已知限制、GitHub 安全、文档入口 |
| `server/README.md` | 重写：目录说明、11 个 cmd 入口表、依赖/配置、启动顺序、测试（含集成/race）、网络边界、数据库；删除过期的 `api/ … (user, group)` 表述 |
| `web/README.md` | 重写：启动/构建/测试、7 个页面（含 `/contacts` 好友闭环）、行为边界（isOnline 非实时、pending 不持久化、仅文本）、E2E 说明 |
| `admin/README.md` | 重写：启动/构建/测试、8 个页面、权限与安全；Dashboard 明确为真实 `/api/settings/admin/overview` |
| `docs/validation-status.md` | 新建：最近一次真实验证（2026-11-11）、实测结果表、NOT RUN/SKIP 清单（race/Compose/Kafka/性能/E2E）与复现命令、历史记录 |
| `docs/known-issues.md` | 新建：待补验证 V-1..V-5、业务限制 K-1..K-10、工程限制 E-1..E-6；供 README 与后续轮次引用 |

## 一致性核对

- 根 README 的 9 个服务启动命令与 `server/cmd/` 实际目录一一对应；`migrate` 参数与 `cmd/migrate` 的 `-env/-command/-steps` 一致。
- Web 页面清单与 `web/src/router/index.ts` 一致（login/register/home/contacts/chat/group/files/profile）。
- Admin 页面清单与 `admin/src/router/index.ts` 一致（login/dashboard/users/chats/groups/files/settings/logs/forbidden）。
- 根 README 的 17 service 数与 `docker compose config --services` 一致。
- 过期表述扫描（`later checkpoint|remain later|remain later work|16 files|90 tests|Dashboard metrics remain`）：除 `docs/performance/baseline.md` 中带“2026-10-15 时点”说明的历史数字外，无命中。

## 测试 / 验收

本任务只改文档：

```text
git diff --check                  -> PASS
go build ./...                    -> PASS
docker compose config --quiet     -> PASS
```

前端未改动，无需 vue-tsc/vitest/build。

## 问题与修复

- 根 README 需要引用验证状态与已知问题，但两个文件尚不存在；本轮一并创建，避免死链。
- `server/README.md` 原写 `api/ … (user, group)`，与删除 group-rpc 后的事实不符；已改为仅 user。
- 未发现需要修复的业务代码问题；本任务只改文档。

## Git

- commit：`docs: rewrite readmes to current state`（本记录所在提交）。
- push：随本轮各任务提交推送。

## 下一步

Task 3：代码整洁（孤儿/死代码、未读取配置键、过期注释）。
