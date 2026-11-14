# 001 / Task 0 — 创建并推送 gim-v1.2

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = origin/main = c97618f`，工作区干净；`gim-v1`、`gim-v1.1` 存在且未移动。
- **结果**：`gim-v1.2` annotated tag 创建并推送成功。

## Tag 创建

```powershell
git tag -a gim-v1.2 -m "GIM V1.2: image message Phase 1 (protocol, backend, web, authorization, race, group E2E); file/voice/video/recall V2 pending"
```

- 指向 commit：`c97618f`（图片 Phase 1 收尾的最终 commit）。
- `git push origin main`：`Everything up-to-date`（main 已同步）。
- `git push origin gim-v1.2`：`* [new tag] gim-v1.2 -> gim-v1.2`。

## 远端校验（git ls-remote）

```text
refs/tags/gim-v1     3db4382ae6164e888ac979aa5423c0543c4d5143
refs/tags/gim-v1^{}  09994cb63b1c11f464ac53f08363344fb2612160
refs/tags/gim-v1.1   87949b56348ac84193c5c4865d437cf1abbe8bae
refs/tags/gim-v1.1^{} 55bbb8e36df0ea1f291c22c125129c8e64a8909d
refs/tags/gim-v1.2   a2f5885d1a074b59d08cf9c73a99a565bda2c4af
refs/tags/gim-v1.2^{} c97618f6a6a8545b2ace3c3e4c6ff9a20356d59c
```

- 三个 tag 同时在远端；`gim-v1` 仍为 `09994cb`、`gim-v1.1` 仍为 `55bbb8e`，未移动。
- 未使用 `--force`；未删除任何 tag。

## 验收

- `gim-v1.2`：**创建并推送成功**。
- 下一步进入文件消息协议设计。
