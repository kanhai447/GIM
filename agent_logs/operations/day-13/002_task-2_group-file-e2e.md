# 002 / Task 2 — 群聊文件浏览器 E2E

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = 4c4a2cc`；Compose 全栈可用（17 services：15 healthy + 2 exit 0）。
- **结果**：群聊文件消息 **PASS**（22 步，含下载与授权）。

## 步骤与真实输出

Chrome + Playwright，Compose Web `http://127.0.0.1:5173`；下载使用 `cache: 'no-store'` 避免 E-9 缓存假失败。

```text
STEP 1-4 register A / B / C / D
STEP 5-8 login A / B / C / D
STEP 9 A resolved B=291 C=292
STEP 10 A created group 18 and invited B/C
STEP 11 B opened the group
STEP 12 C opened the group
STEP 13 A uploaded and sent a group file message
STEP 14-15 B received the group file card
STEP 14-15 C received the group file card
STEP 16-17 B downloaded report.pdf
STEP 16-17 C downloaded report.pdf
STEP 18 resolved fileID=67
STEP 19 D (non-member) got 2503
STEP 20 B left the group
STEP 21 B (left group) got 2503
STEP 22 A (owner) still downloaded report.pdf
E2E RESULT=PASS
```

## 覆盖

- 群成员 B/C 收到 `group.message`（type=3）并下载 `report.pdf`。
- 非群成员 D 下载统一 `2503`。
- B 退群后下载被拒；owner A 仍可下载（所有者语义）。
- 未测试 owner 退群（`LeaveGroup` 禁止 owner 普通退出）；未测试解散群。

## 问题与修复

- 无。复用图片 E2E 的 `cache: 'no-store'` 经验，一次通过。

## 环境清理

- `docker compose down`（volumes 保留）；Docker Desktop 停止；临时 E2E 脚本删除。

## 验收

- 群聊文件 E2E：**PASS**；未修改业务代码。

## 日志

- 本文件。
