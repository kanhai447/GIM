# 003 / Task 3 — 打 gim-v2.1 tag 并 push

- **日期**：2026-11-14（Asia/Shanghai）
- **前置**：`56da568`（K-3 修正）、`56d279d`（README 索引）已提交并推送。
- **结果**：annotated tag `gim-v2.1` 创建并推送成功；前六个 tag 未移动。

## 操作与真实结果

```text
git push origin main       2b9d777..56d279d main -> main   exit 0
git tag -a gim-v2.1 ...    exit 0
git push origin gim-v2.1   [new tag] gim-v2.1 -> gim-v2.1   exit 0
```

- tag object：`218c65b2d7cc3fbbc8d34040b2a350687941776d`
- target commit：`56d279ddd054b7dad6f27b09280187b6c788df8b`（`docs: index architecture and interview docs in readme`，包含 K-3 修正 + README 索引）

## 远端核验（peeled commit）

| tag | peeled commit | 状态 |
|---|---|---|
| `gim-v1` | `09994cb63b1c11f464ac53f08363344fb2612160` | 未变 |
| `gim-v1.1` | `55bbb8e36df0ea1f291c22c125129c8e64a8909d` | 未变 |
| `gim-v1.2` | `c97618f6a6a8545b2ace3c3e4c6ff9a20356d59c` | 未变 |
| `gim-v1.3` | `32a4a55d72bd7aa31205d33d378f8e2475b7436f` | 未变 |
| `gim-v1.4` | `886d55ec248ba060bcc059aba821fe97cc4239fd` | 未变 |
| `gim-v2.0` | `8e3966821af989185b1515c665b9a647b61c74f8` | 未变 |
| `gim-v2.1` | `56d279ddd054b7dad6f27b09280187b6c788df8b` | 新增 |

- 未使用 `git tag -f`/`--force`，未删除 tag。
- tag 指向包含 K-3 修正与 README 索引的 commit；release 记录随后提交，不移动 tag。
