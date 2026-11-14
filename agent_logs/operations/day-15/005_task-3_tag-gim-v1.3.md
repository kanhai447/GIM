# 005 / Task 3 — 打 gim-v1.3 tag 并 push

- **日期**：2026-11-14（Asia/Shanghai）
- **前置**：`cb0556c`（撤回收尾测试/证据）与 `32a4a55`（好友上线提醒协议）已提交；main 已推送。
- **结果**：annotated tag `gim-v1.3` 创建并推送成功；旧 tag 未移动。

## 操作

```powershell
git push origin main
git tag -a gim-v1.3 -m "GIM V1.3: image/file/recall messages implemented and validated; friend online reminder protocol defined, implementation pending"
git push origin gim-v1.3
git ls-remote --tags origin
```

## 真实结果

- main：`e3e37fd..32a4a55 main -> main`，exit 0。
- tag：`gim-v1.3` annotated tag object `a5ed44f0754eef2e41175979adee850bff7f86ac`，target commit `32a4a55d72bd7aa31205d33d378f8e2475b7436f`（`docs: define friend online reminder protocol`）。
- tag push：`[new tag] gim-v1.3 -> gim-v1.3`，exit 0。

## 远端核验

| tag | peeled commit | 状态 |
|---|---|---|
| `gim-v1` | `09994cb63b1c11f464ac53f08363344fb2612160` | 未变 |
| `gim-v1.1` | `55bbb8e36df0ea1f291c22c125129c8e64a8909d` | 未变 |
| `gim-v1.2` | `c97618f6a6a8545b2ace3c3e4c6ff9a20356d59c` | 未变 |
| `gim-v1.3` | `32a4a55d72bd7aa31205d33d378f8e2475b7436f` | 新增 |

- 未使用 `git tag -f`、`git push --force`，未删除任何 tag。

## 备注

- tag 指向本轮功能/文档最终验收 commit `32a4a55`；随后提交的 release 记录（本文件、day-15、CURRENT_STATE）不移动 tag。
