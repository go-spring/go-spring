# Bug 30：OnOnce 零条件 Panic

- 状态：已提交
- 位置：`gs/gs.go`
- 预期来源：一致性行为。`And()`、`Or()`、`None()` 的零参数调用用于表达“无条件”，`OnOnce` 包装这些条件时也不应返回一个后续会 panic 的条件。
- 问题：`OnOnce()` 会返回一个非 nil 条件，并在第一次 `Matches` 时调用 `gs_cond.And().Matches(ctx)`，触发 nil 条件方法调用；`OnOnce(nil)` 也没有在入口处立即拒绝 nil 条件。
- 影响：用户组合条件时传入空条件集或 nil 条件，会在容器刷新阶段暴露不可定位的 panic。
- 复现：新增 `TestOnOnce/no conditions` 和 `TestOnOnce/nil condition`。
- 修复：`OnOnce` 在零参数时直接返回 nil；非空时先构造并校验底层条件，再缓存其第一次匹配结果。
- 验证：
  - `go test ./gs -run 'TestOnOnce'`：修复前失败，复现零条件返回非 nil 条件、nil 条件未拒绝；修复后通过。
  - `go test ./gs/internal/gs_cond`：通过。
  - `go test ./...`：通过。
- 提交：本提交 `fix OnOnce empty conditions`。
- 备注：保留 `OnOnce` 对非空条件只执行一次的行为。
