# 006 / Task 5 — 打 gim-v1.4 tag 并 push

- **日期**：2026-11-14（Asia/Shanghai）
- **前置**：`db625c5`（后端）、`6f8ea2d`（前端）、`886d55e`（文档）已提交并推送。
- **结果**：annotated tag `gim-v1.4` 创建并推送成功；旧 tag 未移动。

## 操作与真实结果

```text
git push origin main        eace570..886d55e main -> main   exit 0
git tag -a gim-v1.4 ...     exit 0
git push origin gim-v1.4    [new tag] gim-v1.4 -> gim-v1.4   exit 0
```

- tag object：`2cf99b51c41da8d15a61496657c45e62aa5d61d6`
- target commit：`886d55ec248ba060bcc059aba821fe97cc4239fd`（`docs: record friend online reminder support`）

## 远端核验

| tag | peeled commit | 状态 |
|---|---|---|
| `gim-v1` | `09994cb63b1c11f464ac53f08363344fb2612160` | 未变 |
| `gim-v1.1` | `55bbb8e36df0ea1f291c22c125129c8e64a8909d` | 未变 |
| `gim-v1.2` | `c97618f6a6a8545b2ace3c3e4c6ff9a20356d59c` | 未变 |
| `gim-v1.3` | `32a4a55d72bd7aa31205d33d378f8e2475b7436f` | 未变 |
| `gim-v1.4` | `886d55ec248ba060bcc059aba821fe97cc4239fd` | 新增 |

- 未使用 `git tag -f`、`--force`，未删除任何 tag。
- tag 指向功能/文档最终验收 commit；随后 release 记录不移动 tag。
