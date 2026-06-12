# Bug 13：BeanDefinition 参数类型未校验

- 状态：已提交
- 位置：`gs/internal/gs_bean/bean.go`
- 预期来源：一致性行为；`BeanDefinition` 实现了 `gs.Arg`，作为构造参数时应像 `ValueArg`、`BindArg` 一样校验返回值能赋给目标参数类型。
- 问题：`BeanDefinition.GetArgValue` 直接返回自身 `reflect.Value`，忽略调用方传入的目标类型 `t`。
- 影响：用户把不兼容的 bean 显式传给构造函数参数时，参数解析阶段不会返回错误，后续 `reflect.Call` 可能 panic。
- 复现：扩展 `TestBeanDefinition/normal`，对 `*TestBean` bean 请求 `*bytes.Buffer` 参数值，修复前不会返回错误。
- 修复：在 `BeanDefinition.GetArgValue` 中检查 bean value 类型是否可赋给目标类型，不可赋值时返回错误。
- 验证：`go test ./gs/internal/gs_bean -run 'TestBeanDefinition/normal'` 通过；`go test ./gs/internal/gs_bean` 通过；`go test -race ./gs/internal/gs_bean` 通过；`go test ./...` 通过；`go vet ./...` 通过。
- 提交：本提交 `validate bean definition arg type`。
- 备注：与 Bug 11 的返回类型校验保持错误信息一致。
