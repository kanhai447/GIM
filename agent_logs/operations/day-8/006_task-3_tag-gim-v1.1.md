# 006 / Task 3 — 创建 gim-v1.1 tag（本地成功，push 失败）

- **日期**：2026-11-12（Asia/Shanghai）
- **开始状态**：`HEAD = 55bbb8e`（`docs: update validation status after round-5 checks`），`origin/main = bce78cc`。
- **结果**：本地 annotated tag 创建成功；main 与 tag 的 push 均因网络不可达失败。

## Tag 创建

```powershell
git tag -a gim-v1.1 -m "GIM V1.1: project tidy-up, friend loop, C1/C2 fixes, validation status" ...
```

Tag message 如实写明：

- 已验证：单测、真实依赖集成（20 PASS / 1 SKIP，SKIP 的 Kafka 用例已由 Compose broker 补齐）、Linux Docker race PASS、Compose 17 services、Kafka produce/consume/restart/幂等 PASS、浏览器好友 E2E 11 步 PASS、性能最小两档 PASS。
- 未验证：完整性能阶梯（25/50/100/200 连接）、5 分钟 soak、restart 稳定性、文件吞吐、远端 GitHub Actions；性能数字为单次基线，不是容量声明。

## Tag 指向与旧 tag 校验

```text
gim-v1    -> 09994cb63b1c11f464ac53f08363344fb2612160   （未移动）
gim-v1.1  -> 55bbb8e36df0ea1f291c22c125129c8e64a8909d   （本轮验证状态提交）
```

`git tag` 同时存在 `gim-v1` 与 `gim-v1.1`；`gim-v1` 的 hash 与第四轮记录一致（09994cb）。

## Push 真实结果（失败）

尝试顺序与真实错误：

```text
git push origin main
  attempt 1 -> fatal: unable to access 'https://github.com/kanhai447/GIM.git/': Recv failure: Connection was reset
  attempt 2 -> fatal: ... Failed to connect to github.com port 443 after 21060 ms: Could not connect to server
  attempt 3 -> fatal: ... Could not connect to server
  等待 60s 后重试 -> fatal: ... Could not connect to server

git push origin gim-v1.1
  -> fatal: unable to access 'https://github.com/kanhai447/GIM.git/': Recv failure: Connection was reset
```

- 未使用 `git push --force`；未执行 `git tag -f`；未删除/移动 `gim-v1`。
- 当前 Local/Remote：`HEAD = 55bbb8e`，`origin/main = bce78cc`；本地存在 `gim-v1.1`，远端尚未收到。

## 待用户决定

按本轮第 8 条，tag 推送失败需要确认：是否保留本地 `gim-v1.1` 并稍后重试 `git push origin main && git push origin gim-v1.1`，还是调整网络/认证后重新推送。**没有远端 tag 被覆盖或删除。**

## 恢复命令（网络可用后）

```powershell
git push origin main
git push origin gim-v1.1
git ls-remote --tags origin
```

预期：`origin/main` = `55bbb8e`，远端 tags 同时包含 `gim-v1`（09994cb）与 `gim-v1.1`（55bbb8e）。
