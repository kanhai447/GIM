# Docker Compose（单机开发环境）

本配置在一张 Docker bridge network 上运行 GIM V1 全栈：MySQL、Redis、单节点 KRaft Kafka、单节点 etcd、migration、User/Group RPC、七个 API/Consumer、Gateway，以及由非特权 Nginx 提供的 Web/Admin production build。

## Prerequisites

- Docker Desktop（Windows 使用 Linux containers）
- Docker Compose v2 或 Docker Desktop 内置的 `docker compose`
- 建议至少 8 GiB 可用内存

## Setup

从仓库根目录复制本地配置模板：

```powershell
Copy-Item .env.compose.example .env.compose.local
```

将所有 `CHANGE_ME` 替换为本机开发值。`.env.compose.local` 已被 Git 忽略；不要提交它。MySQL、Redis 与 JWT 值必须由运行者设置，JWT Secret 应使用密码学安全的随机生成器生成（例如 `openssl rand -base64 48`），不能沿用模板文字或弱默认值。

应用容器使用 Docker DNS 名称访问 `mysql`、`redis`、`etcd` 与 `kafka`。浏览器只通过 Web/Admin 同源 `/api` 反向代理进入 Gateway，不直接访问内部 API。

## Start

```powershell
docker compose up -d --build
docker compose ps
```

首次启动由一次性 `migration` job 在 MySQL healthy 后应用 001–008；业务容器只在 migration 成功及其关键依赖 healthy 后启动。重复启动会幂等检查已应用版本。`storage-init` 只准备 File volume 权限。

公开入口：

- Web：`http://127.0.0.1:5173/`
- Admin：`http://127.0.0.1:4174/`
- Gateway：`http://127.0.0.1:8080/`

MySQL、Redis、etcd 与 Kafka 的调试端口只绑定到 loopback。业务 API/RPC 不映射到宿主机。

## Initial administrator

全新数据库不会内置管理员或密码。通过当前 shell 的临时环境变量提供账号、昵称与密码，然后执行一次性工具：

```powershell
docker compose --profile tools run --rm `
  -e GIM_ADMIN_ACCOUNT -e GIM_ADMIN_NICKNAME -e GIM_ADMIN_PASSWORD `
  admin-bootstrap
```

该命令复用正式 bcrypt、User Service、Domain role/status 与 Repository，只创建启用的 admin；账号已存在时会失败，不会在每次启动时重置密码。不要把实际变量值写入仓库、脚本或操作日志。

## Status and logs

```powershell
docker compose ps
docker compose logs -f <service>
```

本地性能验证时，Chat、Group、File 和 Logs 可在各自容器内启用仅监听
`127.0.0.1:6060` 的 debug listener。它不发布到宿主机，也不经过 Gateway。
当前 Compose 配置启用该监听，供 `scripts/perf-snapshot.ps1` 采集 goroutine、
heap、数据库连接池和 audit producer queue/drop 计数；Compose 外将
`GIM_DEBUG_ADDR` 留空即可禁用。

长期运行容器配置 `unless-stopped`；migration、storage-init 与手动 bootstrap 不自动重启。应用日志写 stdout/stderr，Kafka audit 数据单独进入 `audit_logs`。

## Stop and persistence

```powershell
docker compose down
```

普通 `down` 保留 MySQL、Redis、etcd、Kafka 和 File named volumes。File 的 `temp` 与 `objects` 同处 `file_storage` 挂载根目录，保持 same-filesystem atomic rename 语义。

彻底重置本 Compose 项目的开发数据：

```powershell
docker compose down -v
```

警告：`down -v` 会删除本 Compose 项目的全部开发数据库、消息、文件、Kafka 与 etcd 数据；不会删除宿主机其他数据库或非本项目 volumes。

## Build and validation

```powershell
docker compose config --quiet
docker compose build
```

Go 使用多阶段静态构建，runtime 不含 Go toolchain；Web/Admin 使用 `pnpm build` 后复制 `dist` 到非特权 Nginx，runtime 不含 source、pnpm cache 或 node_modules。`.dockerignore` 排除 Git、reference、agent logs、本地环境文件、依赖与构建产物。

## Known limitations

- 仅面向 development/single-host；无 HA。
- MySQL、Redis、etcd 与 Kafka 都是单节点；Kafka 为单 broker KRaft。
- 内部服务间无 TLS，也未接入 production secret manager。
- 当前未设置 Compose CPU/memory hard limit，避免把开发机限制过死。
- audit 是 best-effort，无 transactional outbox；Kafka 故障期间允许丢失 audit，新事件在恢复后继续。
- 已完成单机负载与 5 分钟稳定性基线，但未进行多机、云环境或容量上限测试；最大已测值不是容量声明。
- Windows native race 因当前 32-bit MinGW 不兼容而不可用；Linux Docker full/focused/real-dependency race 均 PASS，项目 Race Verification 已在兼容 Linux 环境完成。
- Chat/Group 当前仅支持文本消息；无 receiver delivery/read receipt，File 尚未接入消息协议。
- 浏览器下载会物化 Blob；删除最后一个文件引用后，物理孤儿对象等待后续 GC。
- 高压重启存在 DB commit 与 ACK/live publish 之间的窗口；幂等重试与历史可恢复，但不承诺实时 exactly-once。
- GitHub Actions workflow 已配置，最终本地等价检查 PASS；远端执行结果在收口环境中为 NOT OBSERVED。
