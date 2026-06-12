# Bug 21：NewBean 无效 Nil 反射 Panic

- 状态：已提交
- 位置：`gs/internal/gs_bean/bean.go`
- 预期来源：`NewBean` 已有 typed nil 测试并预期返回 `bean instance cannot be nil`；公开 `gs.Provide` 最终也依赖该构造入口。
- 问题：`NewBean` 在校验 `reflect.Value` 是否有效前先调用 `v.Type()`，untyped nil 或 `reflect.Value{}` 会触发 `reflect.Value.Type on zero Value` panic。
- 影响：调用方传入 nil bean 时得到底层反射 panic，而不是项目已有的 bean nil 诊断信息。
- 复现：扩展 `TestNewBean/nil_bean_value`，覆盖 `NewBean(nil)` 和 `NewBean(reflect.Value{})`。
- 修复：在读取 `v.Type()` 前先检查 `v.IsValid()`。
- 验证：
  - `go test ./gs/internal/gs_bean -run TestNewBean`：修复前新增 nil 场景复现底层反射 panic；修复后通过。
  - `go test ./gs/internal/gs_bean ./gs/internal/gs_core/resolving ./gs/internal/gs_core/injecting ./gs/internal/gs_app ./gs`：通过。
  - `go test ./...`：通过。
  - `go test -race ./gs/internal/gs_bean`：通过。
- 提交：本提交 `validate NewBean nil value`。
- 备注：typed nil 指针的既有错误语义保持不变。
