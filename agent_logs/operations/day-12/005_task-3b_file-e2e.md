# 005 / Task 3B — 浏览器文件消息 E2E

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = 5e2abb3`（前端文件消息）。
- **结果**：普通文件消息浏览器 E2E **PASS**（8 步）。

## 环境

- Compose 全栈重建：17 services = 15 running healthy + `migration`/`storage-init` exit 0。
- Chrome + Playwright，`http://127.0.0.1:5173`（Nginx → Gateway → Chat/File API）。
- 测试文件：`report.pdf`（小型 PDF bytes）。

## 步骤与真实输出

```text
STEP 1 register A / B / D
STEP 2 login A / B / D
STEP 3 A found B (288) and sent request
STEP 4 B found A (287) and accepted
STEP 5 A uploaded and sent a file message
STEP 6 B received the file card
STEP 7 B downloaded report.pdf
STEP 8 D (non-participant) got 2503
E2E RESULT=PASS
```

覆盖：选择文件 → 上传 → `type=3` 发送 → ACK/pending 转 sent → 接收方文件卡片（文件名/大小）→ 带 token 下载（`download` 事件文件名为 `report.pdf`）→ 非参与者 D 下载 `2503`。

## 问题与修复

- 无。文件消息复用图片的 pending/ACK/下载授权路径，E2E 一次通过。

## 环境清理

- `docker compose down`（volumes 保留）；Docker Desktop 停止；临时 E2E 脚本删除。

## 验收

- 浏览器文件消息：**PASS**。

## 日志

- 本文件；未修改业务代码。
