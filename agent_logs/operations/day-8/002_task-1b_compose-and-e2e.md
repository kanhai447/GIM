# 002 / Task 1B — Compose 全栈 + 浏览器好友 E2E

- **日期**：2026-11-12（Asia/Shanghai）
- **开始状态**：`HEAD = bce78cc`，工作区干净；Docker Desktop 已由本轮任务启动。
- **结果**：Compose 17 services 全部符合预期；浏览器好友 E2E **PASS**。

## 1. Docker 环境

- 本轮开始时 Docker daemon 未运行；执行 `Start-Process "C:\Program Files\Docker\Docker\Docker Desktop.exe"` 后轮询 `docker info`，约 3 分钟内就绪：**Docker 29.4.0**。
- `.env.compose.local` 已存在（104 行，除注释外无 `CHANGE_ME`），`docker compose config --quiet` PASS。

## 2. Compose build/up

```powershell
docker compose up -d --build
```

真实状态（`docker compose ps -a`）：

```text
admin        Up (healthy)
auth-api     Up (healthy)
chat-api     Up (healthy)
etcd         Up (healthy)
file-api     Up (healthy)
gateway      Up (healthy)
group-api    Up (healthy)
kafka        Up (healthy)
logs-api     Up (healthy)
migration    Exited (0)
mysql        Up (healthy)
redis        Up (healthy)
settings-api Up (healthy)
storage-init Exited (0)
user-api     Up (healthy)
user-rpc     Up (healthy)
web          Up (healthy)
```

统计：**17 services = 15 running healthy + 2 一次性 job exit 0**。
再次执行 `docker compose up -d` → exit 0（幂等启动确认）。

## 3. 浏览器好友 E2E

环境：Chrome（`C:\Program Files\Google\Chrome\Application\chrome.exe`）+ `playwright-core`；目标 Compose Web `http://127.0.0.1:5173`（Nginx → Gateway → Auth/User）。

流程（临时脚本放在 ignored 的 `web/node_modules/.tmp-friend-e2e.mjs`，跑完删除）：

```text
STEP 1 register A
STEP 2 register B
STEP 3 login A
STEP 4 login B
STEP 5 A searched B
STEP 6 A sent request
STEP 7 B accepted
STEP 8 A sees friend
STEP 9 A saved remark
STEP 10 A deleted friend
STEP 11 B friend list empty
E2E RESULT=PASS
```

覆盖：注册、登录、搜索、申请、同意、好友列表、备注、删除、双方列表同步；全部经真实浏览器与 Compose 全栈。

## 4. 问题与修复

- 首轮 `docker compose up` 的 PowerShell 管道退出码为 1（stderr 被 PowerShell 视为错误 + 前端健康尚未完成），但容器实际正常启动；用 `docker compose ps` 与页面探测（5173/4174 均 200）确认，随后 `docker compose up -d` 复跑 exit 0。
- E2E 脚本仅用于本轮验证，位于 `web/node_modules/`（Git ignored），验证后删除，不进入仓库。

## 5. 验收

- Compose 全栈：**PASS**（17 services，15 healthy + 2 exit 0）。
- 浏览器好友 E2E：**PASS**（11 步）。

## 6. 日志

- 本文件；Docker/Compose 未修改仓库文件。
