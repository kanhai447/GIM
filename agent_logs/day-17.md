# Day 17 — V2 收口 + gim-v2.0（2026-11-14）

## 今日目标

新建 V2 范围文档，统一 README/validation/known-issues/子 README 表述，打 `gim-v2.0` 并 push；不改业务代码。

## 真实结果

| 项目 | 结果 |
|---|---|
| `docs/v2-scope.md` | 新建：已实现四 Phase、V2 后期/不做及理由、K-14/K-15/K-16/E-8/E-9、验证摘要、tag 语义、简历表述 |
| README / validation / known-issues / docs 04–07 / 子 README | 统一链接 v2-scope；未删除 NOT RUN |
| `go build ./...` | PASS（无代码变更，确认无意外） |
| `git diff --check` | PASS |
| 文档矛盾 | 未发现无法消解冲突；无“全量验证通过/百万连接”声明 |
| tag `gim-v2.0` | annotated object `71032d5`，target `8e39668`；已推送 |

## Commit / Tag

- `6d3e146` docs: define V2 scope and boundaries
- `8e39668` docs: align V2 scope references
- annotated `gim-v2.0`：object `71032d5`，target `8e39668`
- main push `abc3e72..8e39668`；tag push new `gim-v2.0`
- 旧 tag peeled commit 未变：09994cb / 55bbb8e / c97618f / 32a4a55 / 886d55e

## 未验证项（与 v2-scope 一致）

- 完整性能阶梯、5 分钟 soak/restart、图片/文件吞吐与预览性能：NOT RUN。
- 远端 GitHub Actions：NOT OBSERVED。
- 多实例 Presence、user_confs、管理员撤回、好友申请实时推送：范围外，未实现。
- K-14 严格单条撤回需 schema 决策；K-15 单机好友提醒；K-16 offline best-effort；E-8/E-9 边界。

## 下一步

到此为止，或按需授权：user_confs / 管理员撤回 / 性能 / 多实例 Presence。
