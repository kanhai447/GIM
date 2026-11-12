# Day 8 — 补验证、更新验证状态、打 gim-v1.1 tag（2026-11-12）

## 今日目标

尽量补 race / Compose / Kafka / 性能 / 浏览器 E2E 验证；更新验证状态文档；创建 `gim-v1.1` annotated tag 并 push，不移动 `gim-v1`。

## 真实结果

| 项目 | 结果 | 环境 / 命令 |
|---|---|---|
| race（全量） | **PASS** | `golang:1.25-bookworm` + 挂载 module cache + `GOPROXY=off`，`go test -race ./... -count=1` exit 0，无 race warning |
| Compose 全栈 | **PASS** | Docker 29.4.0；`docker compose up -d --build`；17 services = 15 healthy + 2 一次性 job exit 0 |
| Kafka 真实消费 | **PASS** | Compose Kafka 4.2.2 KRaft，`GIM_KAFKA_INTEGRATION=1`，2 个用例通过 |
| 浏览器好友 E2E | **PASS** | Chrome + Compose Web，11 步（注册→申请→同意→备注→删除） |
| 性能最小场景 | **PASS（仅两档）** | HTTP profile c10/10s 27281/27281、p95 4.834 ms；Private 10 conn/10s@10 100/100 ACK、delivery 100/100、p95 22.859 ms |
| 完整性能阶梯 / soak / restart / 文件吞吐 | **NOT RUN** | 未复跑；历史数字仍为 2026-10-15 |
| 远端 GitHub Actions | **NOT OBSERVED** | 本环境无 API 访问 |

## 关键过程

1. 本轮开始时 Docker 未运行；启动 Docker Desktop 后 daemon 就绪（29.4.0）。
2. 本机 Windows native race 仍失败（32-bit MinGW 不兼容；64-bit MinGW 缺 libc headers）；WSL 无 Go/gcc 且 sudo 需密码；最终在 Linux 容器内完成。
3. race 容器首次因 `proxy.golang.org` 不可达而 `[setup failed]`（非 race 失败）；挂载本机 module cache、`GOPROXY=off` 后全量 PASS。
4. Compose 全栈启动并等待 healthy；首次 `up` 的 PowerShell 管道退出码为 1 属 stderr/健康时序问题，`docker compose up -d` 复跑 exit 0、17 services 确认。
5. 浏览器 E2E 使用临时 Playwright 脚本（`web/node_modules/`，ignored，跑完删除），11 步 PASS。
6. Kafka 用 Compose broker（宿主 19092）跑真实 produce/consume/restart/幂等用例，PASS。
7. 性能用 `server/tools/loadtest` 复跑 HTTP profile 与 Private 两档；首轮因 prepare/workload 密码不一致 1201，统一密码后 PASS；cleanup 删除 20 users/3 groups。
8. 结束后：`docker compose down`（保留 volumes）、停止 Docker Desktop、停止临时 etcd、删除临时 E2E 脚本与含凭据的 host env 文件。

## 修改文件

- `docs/validation-status.md`：新增 2026-11-12 验证矩阵与证据；未验证项收窄。
- `docs/known-issues.md`：V-1/V-2/V-3/V-5 移入“已验证”；保留 V-4（完整性能/soak）、V-6（远端 CI）。
- `README.md`：验证状态与 tag 说明更新（`gim-v1.1` 包含 race/Compose/Kafka/E2E，不含完整性能阶梯/soak）。
- `agent_logs/operations/day-8/`：001–005 操作记录。
- `agent_logs/prompts/round-5.md`：本轮指令。

## 问题与修复

1. race 容器模块下载失败 → 挂载 module cache + `GOPROXY=off`。
2. Compose 首次 `up` 管道退出码 1 → 以 `docker compose ps` 与页面探测确认实际健康，复跑 `up -d` exit 0。
3. 性能首轮 1201 → prepare 与 workload 使用同一 `GIM_PERF_PASSWORD`。
4. 未发现业务缺陷，未修改业务代码。

## 当日验收

- 文档与 tag 说明如实反映：功能完成 + 集成测试 PASS + race/Compose/Kafka/E2E 已验证 + 性能仅最小两档。
- `gim-v1` 未移动；本轮新建 `gim-v1.1`。

## 次日入口

- 可选：完整性能阶梯/soak/restart/文件吞吐复跑；随后进入 V2（富消息、好友上线提醒、`user_confs`）。
