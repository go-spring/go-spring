# Bug 26：Lifecycle Nil 函数延迟 Panic

- 状态：已提交
- 位置：`gs/internal/gs_bean/bean.go`
- 预期来源：`Init`/`Destroy` 是 bean 生命周期注册入口，已有测试要求非法签名在注册阶段 panic；nil 函数也应在注册阶段被拒绝。
- 问题：untyped nil 会在 `reflect.TypeOf(fn)` 后进入 `typeutil.IsFuncType(nil)` 触发底层反射 panic；typed nil 函数会通过签名校验并保存，直到生命周期调用时才 panic。
- 影响：用户误传 nil init/destroy 函数时，错误位置远离注册点，可能在容器启动或关闭阶段崩溃。
- 复现：扩展 `TestBeanDefinition/init_function` 与 `destroy_function`，覆盖 untyped nil 和 typed nil 函数。
- 修复：`Init`/`Destroy` 在签名校验前先拒绝 nil 函数。
- 验证：
  - `go test ./gs/internal/gs_bean -run TestBeanDefinition`：修复前 untyped nil 复现底层 panic，typed nil 未 panic；修复后通过。
  - `go test ./gs/internal/gs_bean ./gs/internal/gs_core/injecting ./gs/internal/gs_core/resolving ./gs/internal/gs_app ./gs`：通过。
  - `go test ./...`：通过。
  - `go test -race ./gs/internal/gs_bean`：通过。
- 提交：本提交 `reject nil lifecycle functions`。
- 备注：不改变非 nil 非法签名的既有错误文本。
