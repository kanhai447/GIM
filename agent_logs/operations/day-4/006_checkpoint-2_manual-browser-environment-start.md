# Day 4 / Checkpoint 2 — Manual Browser Environment Start

## 时间

- 2026-10-14 16:39 +08:00

## 目标

- 仅启动 Admin Final Integration 的本地 Manual Browser E2E 完整环境。
- 完成最小健康检查后保持服务运行，等待用户手工测试。

## 范围与安全

- 保留既有 Day 4 / Checkpoint 2 未提交实现；未执行 reset、clean、commit 或 push。
- 未启动 Docker Compose，未调用 Browser Computer Use，未进入下一 Checkpoint。
- 仅调整 ignored `.env.local` 中缺失的本地非敏感运行项：Group RPC 端口、Kafka audit topic、Admin 本地 origin；未修改正式/生产配置。
- 本记录不包含账号密码、密码哈希、JWT、数据库凭据或其他 Secret。
- Admin Vite 仅绑定 `127.0.0.1:4174`，Gateway 绑定 `127.0.0.1:8080`；内部服务仅供本机 E2E。

## 已启动服务

- 基础设施：MySQL、Redis、Kafka 4.2.2 单节点 KRaft、etcd。
- RPC：User RPC、Group RPC。
- API：User、Auth、Chat、Group、File、Settings、Logs。
- Logs API 内置 Kafka Consumer。
- Gateway。
- Admin Vue/Vite frontend。

## 本地运行方式

- MySQL/Redis 使用已有本地 Windows 服务。
- Kafka 使用 `.tmp/` 下临时下载并解压的官方运行包与本地 KRaft 数据目录；topic 为 `gim.audit.events`。
- etcd 复用 `.tmp/admin-e2e-etcd` 本地单节点数据目录。
- 当前后端源码重新构建到 ignored `.tmp/admin-e2e-bin` 后启动。
- Admin 通过 Vite dev server 启动，并将 `/api` 代理到本地 Gateway。

## 健康检查

- Admin frontend `http://127.0.0.1:4174/`：HTTP 200，app shell PASS。
- Admin Vite `/api` -> Gateway proxy：PASS。
- Gateway `http://127.0.0.1:8080`：公开 Settings 请求经 Gateway -> Auth -> Settings PASS。
- Auth API：公开路径 authentication PASS。
- User Admin API：只读列表 PASS。
- Chat Admin API：只读历史列表 PASS。
- Group Admin API：只读群列表 PASS。
- File Admin API：只读文件列表 PASS。
- Settings API：公开配置读取 PASS。
- Logs API：管理员只读列表 PASS。
- Redis：PING PASS。
- MySQL：Windows 服务运行，且所有 DB-backed Admin API 查询 PASS。
- etcd：health PASS；Auth/User/Chat/Group/File/Settings/Logs 七个 API lease 注册全部存在。
- Kafka：broker topic leader/ISR 正常。
- Kafka -> Logs：发送一条不含账号或凭据的合成 Auth 失败审计事件，随后由 Logs Consumer 持久化并通过 request ID 从 Logs API 查询成功，pipeline PASS。
- 启动后所有 GIM `run2`、Kafka、etcd 与 Admin Vite error log 均为空。

## 问题与处理

- Admin Vite 首次在默认沙箱中启动时，esbuild 子进程被 Windows 以 `EPERM` 拒绝；按权限流程在沙箱外重试。
- 第二次通过 `pnpm run dev -- --host ...` 产生多余的 `--` 参数，Vite 未监听；改用 `pnpm exec vite --host 127.0.0.1 --port 4174 --strictPort` 后 PASS。

## Git

- commit：无。
- push：无。
- 服务继续运行；本操作不创建 Checkpoint commit。

## 下一步

- 等待用户本人在 `http://127.0.0.1:4174/` 执行 Manual Browser E2E。
- 未授权前不继续 Browser E2E、不关闭服务、不进入后续 Checkpoint。
