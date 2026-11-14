# 005 / Task 4 — 群聊图片 WS E2E

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = d699c87`；Compose 全栈可用（17 services：15 healthy + 2 exit 0）。
- **结果**：群聊图片 **PASS**；第 21 步首轮“200”确认为浏览器缓存，服务器授权语义正确。

## 步骤与真实输出

Chrome + Playwright，Compose Web `http://127.0.0.1:5173` → Nginx → Gateway → Group API/File API：

```text
STEP 1-4 register A
STEP 5-8 login A
STEP 1-4 register B
STEP 5-8 login B
STEP 1-4 register C
STEP 5-8 login C
STEP 1-4 register D
STEP 5-8 login D
STEP 9 A resolved B=284 C=285
STEP 10 A created group 17 and invited B/C
STEP 11 B opened the group
STEP 12 C opened the group
STEP 13 A uploaded and sent a group image
STEP 14-15 B received the group image
STEP 14-15 C received the group image
STEP 16 B opened and closed the group image preview
STEP 17 resolved fileID=65
STEP 18 B (member) downloaded the image
STEP 19 D (non-member) got 2503
STEP 20 B left the group
STEP 21 B (left group) got 2503
STEP 22 A (owner) still downloaded the image
E2E RESULT=PASS
```

## 首轮“失败”排查（不是服务端缺陷）

首轮在 B 退群后下载仍返回 `200 image/png`。直查 MySQL 授权条件：

```text
user_files.id=64 -> owner 279
group_messages -> 1 行，admin_deleted_at NULL
group_hits (user 280 in group_members) = 0
chat_hits  (user 280)                  = 0
```

服务器会拒绝；首轮是浏览器 HTTP 缓存返回了 B 作为成员时下载过的同一 URL。E2E 改为 `fetch(..., { cache: 'no-store' })` 后，第 21 步得到 `2503`，第 22 步 owner 仍 `200`。

**结论**：服务端授权正确；已下载到本地的图片副本不会因退群被浏览器删除，这是缓存语义，不是服务端放行。文档记录该边界。

## 覆盖

- 群成员 B/C 收到 `group.message` 并可下载/预览。
- 非群成员 D 下载统一 `2503`。
- 成员 B 退群后下载被拒；owner A 仍可下载。
- 未测试 owner 退群（`LeaveGroup` 明确禁止 owner 普通退出；如需测试解散群，另立用例）。

## 环境清理

- `docker compose down`（volumes 保留）；Docker Desktop 停止。
- 临时 E2E 脚本与 SQL 文件删除，未提交凭据。

## 验收

- 群聊图片 E2E：**PASS**。
- 未发现需要修改 Hub/广播/session 的缺陷。

## 日志

- 本文件；未修改业务代码。
