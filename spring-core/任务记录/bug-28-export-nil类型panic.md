# Bug 28：Export Nil 类型 Panic

- 状态：已提交
- 位置：`gs/internal/gs_bean/bean.go`
- 预期来源：一致性行为。`Export` 对非法导出类型已有明确错误信息，nil `reflect.Type` 也应在 API 边界被识别并给出稳定错误，而不是在调用 `Kind` 时触发反射 panic。
- 问题：`BeanDefinition.Export(nil)` 会直接调用 `t.Kind()`，导致 nil `reflect.Type` 的运行时 panic。
- 影响：用户通过链式注册 API 传入 nil 导出类型时，无法得到可定位的框架错误信息，排查成本高。
- 复现：新增 `TestBeanDefinition/export` 中的 `bean.Export(nil)` 断言。
- 修复：在 `Export` 遍历导出类型时先检查 nil，并 panic 明确错误信息。
- 验证：
  - `go test ./gs/internal/gs_bean -run 'TestBeanDefinition/export'`：修复前失败，复现 nil 指针 panic；修复后通过。
  - `go test ./gs/internal/gs_bean`：通过。
  - `go test ./...`：通过。
- 提交：本提交 `reject nil export type`。
- 备注：仅处理 nil 类型输入，不改变既有非接口类型和未实现接口的校验行为。
