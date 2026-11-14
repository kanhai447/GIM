# 002 / Task 1B — 撤回窗口短配置 E2E

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = e3e37fd`；Compose 全栈可用。
- **结果**：`RECALL_WINDOW_SECONDS=3` 下 **PASS**；已恢复默认配置（无键 → 服务端默认 120s）。

## 配置切换

1. 备份 `.env.compose.local` 到 `.tmp/env.compose.local.bak`（ignored）。
2. 在本地 env 中临时设置 `RECALL_WINDOW_SECONDS=3`。
3. `docker compose up -d --force-recreate chat-api group-api`，两者 healthy。

## E2E 步骤与真实输出

```text
STEP 1 register A
STEP 2 register B
STEP 3 login A
STEP 4 login B
STEP 5 A found B (299) and sent request
STEP 6 B found A (298) and accepted
STEP 7 A recalled immediately within the 3s window
STEP 8 A recall after the 3s window failed with 2220 and the message stayed
STEP 9 B sees the first placeholder and the late message still present
E2E RESULT=PASS
```

覆盖：

- 3s 内撤回成功，B 看到“消息已撤回”，原文不可见。
- 等待 4s 后再次点击撤回：服务端返回 `2220`，前端 `.chat-error` 显示“超过撤回时间窗口”，原消息保留；前端入口仍按 120s UX 显示（服务端权威）。
- 没有出现服务端/前端窗口不一致导致的状态错误。

## 恢复默认配置

- 恢复备份（本地 env 无 `RECALL_WINDOW_SECONDS` 键）。
- `docker compose up -d --force-recreate chat-api group-api`，两者 healthy。
- 容器内 `printenv RECALL_WINDOW_SECONDS` 为空 → 服务端使用默认 120s。
- 临时备份文件删除。

## 验收

- 撤回窗口短配置 E2E：**PASS**。

## 日志

- 本文件；未修改业务代码。
