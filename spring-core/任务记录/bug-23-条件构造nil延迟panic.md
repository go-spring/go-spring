# Bug 23：条件构造 Nil 延迟 Panic

- 状态：已提交
- 位置：`gs/internal/gs_cond/cond.go`
- 预期来源：条件构造器是公开 `gs.OnFunc`、`gs.Not`、`gs.And`、`gs.Or`、`gs.None` 的底层实现；非法 nil 输入应在构造阶段失败，避免条件匹配阶段出现空指针 panic。
- 问题：`OnFunc(nil)`、`Not(nil)`、`And(nil)`、`Or(nil)`、`None(nil)` 会返回可保存的条件对象或 nil 条件，后续调用 `Matches` 时才 panic。
- 影响：条件错误注册位置和最终崩溃位置分离，应用启动排查困难。
- 复现：新增 `TestOnFunc/nil_function`、`TestNot/nil_condition`、`TestAnd/nil_condition`、`TestOr/nil_condition`、`TestNone/nil_condition`。
- 修复：在条件构造入口拒绝显式 nil 函数和 nil 条件；空参数组合的既有语义保持不变。
- 验证：
  - `go test ./gs/internal/gs_cond -run 'TestOnFunc|TestNot|TestAnd|TestOr|TestNone'`：修复前新增 nil 用例均因未 panic 失败；修复后通过。
  - `go test ./gs/internal/gs_cond ./gs/internal/gs_core/resolving ./gs/internal/gs_arg ./gs/internal/gs_bean ./gs`：通过。
  - `go test ./...`：通过。
  - `go test -race ./gs/internal/gs_cond`：通过。
- 提交：本提交 `reject nil conditions`。
- 备注：不改变 `And()`、`Or()`、`None()` 零参数返回 nil 的既有测试预期。
