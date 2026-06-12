# Bug 22：BindArg Nil 函数 Panic

- 状态：已提交
- 位置：`gs/internal/gs_arg/arg.go`
- 预期来源：公开 `gs.BindArg(fn, ...)` 要求传入可调用函数；同包已对非函数和非法签名给出明确错误，不应暴露底层反射 panic。
- 问题：`validBindFunc` 和 `NewCallable` 未校验 untyped nil 与 typed nil 函数，前者会在 `reflect.TypeOf(fn).Kind()` 处 panic，后者会注册 nil 函数并在调用时 panic。
- 影响：用户误传 nil bind 函数时，应用启动或参数解析阶段得到难以定位的反射错误。
- 复现：新增 `TestCallable_New/nil_function` 和 `TestBindArg_Bind/nil_function`。
- 修复：为可调用函数增加统一校验，拒绝 untyped nil、非函数和 typed nil 函数。
- 验证：
  - `go test ./gs/internal/gs_arg -run 'TestCallable_New|TestBindArg_Bind'`：修复前 `NewCallable(nil)` 复现 nil `reflect.Type` panic；修复后通过。
  - `go test ./gs/internal/gs_arg ./gs/internal/gs_bean ./gs/internal/gs_core/injecting ./gs/internal/gs_core/resolving ./gs`：通过。
  - `go test ./...`：通过。
  - `go test -race ./gs/internal/gs_arg`：通过。
- 提交：本提交 `validate callable nil functions`。
- 备注：保留已有非函数和非法签名错误文本。
