# 004 / Task 3B — 浏览器图片 E2E 与接收方下载授权修复

- **日期**：2026-11-12（Asia/Shanghai）
- **开始状态**：`HEAD = b1c1328`（前端图片消息）。
- **结果**：E2E 首轮暴露真实缺陷（接收方无法下载图片），最小修复后 **E2E PASS**。

## 1. 首轮 E2E（失败）

Compose 全栈重建（17 services：15 healthy + 2 exit 0），Chrome + Playwright：

```text
STEP 1 register A
STEP 2 register B
STEP 3 login A
STEP 4 login B
STEP 5 A found B (276) and sent request
STEP 6 B found A (275) and accepted
STEP 7 A uploaded and sent an image
E2E RESULT=FAIL locator.waitFor: Timeout ... [data-testid="image-thumb"]
```

根因：`GET /api/file/:id` 只允许 `user_files.user_id = 当前用户`。图片消息引用发送者的 `user_files.id`，接收方没有该引用 → 下载 `2503`，前端进入“图片加载失败”占位，无法看到图片。

## 2. 最小修复

不新增表/列，不放宽成“任意登录用户可下载”：

- `file/repository` 新增可选接口 `AccessibleRepository.FindAccessibleByID(ctx, userID, fileID)`。
- MySQL 实现：
  - 所有者直接放行；
  - 非所有者仅当存在引用该 `user_files.id` 的图片消息时放行：
    - 私聊：`chat_messages`（`admin_deleted_at IS NULL`，`send_user_id = user` 或 `rev_user_id = user`）；
    - 群聊：`group_messages` JOIN `group_members`（当前用户仍是成员，`admin_deleted_at IS NULL`）；
  - 均用 `JSON_UNQUOTE(JSON_EXTRACT(msg,'$.imageMsg.fileID')) = ?` 匹配。
- `file/service` 新增 `GetAccessible`：数字 ID 走“所有者或消息参与者”，UUID 仍仅所有者。
- `GET /api/file/:id` 改用 `GetAccessible`；非参与者仍返回 `2503`，不泄露文件是否存在。

## 3. 测试

集成测试（真实 MySQL/Redis/WS）新增断言：

```text
receiver 可 FindAccessibleByID(imageFileID)          -> PASS
非参与者（offline user）                            -> ErrNotFound
```

完整图片集成用例：

```text
=== RUN   TestChatImageMessageWebSocketHistoryAndValidation
--- PASS: TestChatImageMessageWebSocketHistoryAndValidation (1.16s)
```

## 4. 修复后 E2E（PASS）

Compose 重建 `file-api` 后重跑：

```text
STEP 1 register A
STEP 2 register B
STEP 3 login A
STEP 4 login B
STEP 5 A found B (278) and sent request
STEP 6 B found A (277) and accepted
STEP 7 A uploaded and sent an image
STEP 8 B received the image
STEP 9 B opened the image preview
STEP 10 B closed the preview
E2E RESULT=PASS
```

## 5. 环境清理

- `docker compose down`（volumes 保留）；Docker Desktop 停止。
- 临时 E2E 脚本（`web/node_modules/.tmp-image-e2e.mjs`）删除，不进入仓库。
- 未提交任何真实配置/凭据。

## 6. 影响与已知边界

- 下载授权现在跨读 `chat_messages`/`group_messages`（同一 MySQL），JSON 条件无索引；文件下载不是高频路径，已登记为工程限制。
- 接收方在消息可见期间可下载；消息被 Admin tombstone 或用户退出群后不再放行。

## Git

- commit：`fix(file): authorize image downloads for message recipients`（本记录所在提交）。
