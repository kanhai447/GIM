# 003 / Task 3 — 打 gim-v2.0 tag 并 push

- **日期**：2026-11-14（Asia/Shanghai）
- **前置**：`6d3e146`（docs/v2-scope）与 `8e39668`（引用对齐）已提交；已推送 main。
- **结果**：annotated tag `gim-v2.0` 创建并推送成功；旧 tag 未移动。

## 操作与真实结果

```text
git push origin main       abc3e72..8e39668 main -> main    exit 0
git tag -a gim-v2.0 ...    exit 0
git push origin gim-v2.0   [new tag] gim-v2.0 -> gim-v2.0   exit 0
```

- tag object：`71032d59f815ae83a0b7369c40bbe536fc7b01fa`
- target commit：`8e3966821af989185b1515c665b9a647b61c74f8`（`docs: align V2 scope references`，包含 `docs/v2-scope.md`）

## 远端核验（peeled commit）

| tag | peeled commit | 状态 |
|---|---|---|
| `gim-v1` | `09994cb63b1c11f464ac53f08363344fb2612160` | 未变 |
| `gim-v1.1` | `55bbb8e36df0ea1f291c22c125129c8e64a8909d` | 未变 |
| `gim-v1.2` | `c97618f6a6a8545b2ace3c3e4c6ff9a20356d59c` | 未变 |
| `gim-v1.3` | `32a4a55d72bd7aa31205d33d378f8e2475b7436f` | 未变 |
| `gim-v1.4` | `886d55ec248ba060bcc059aba821fe97cc4239fd` | 未变 |
| `gim-v2.0` | `8e3966821af989185b1515c665b9a647b61c74f8` | 新增 |

- 未使用 `git tag -f`、`--force`，未删除任何 tag。
- tag 指向包含 V2 范围文档的最终收口 commit；release 记录随后提交，不移动 tag。
