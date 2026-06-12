# Bug 16：匿名指针 Value 标签 Panic

- 状态：已提交
- 位置：`gs/internal/gs_core/injecting/injecting.go`
- 预期来源：运行错误；用户在不支持的字段类型上使用 `value` tag 应返回注入错误，不能触发反射 panic。
- 问题：`wireStruct` 处理带 `value` tag 的匿名字段时，无条件递归调用 `wireStruct(fv, ft.Type, ...)`。如果匿名字段类型是 `*T`，`ft.Type` 不是 struct，递归中调用 `NumField` 会 panic。
- 影响：配置结构中出现匿名嵌入指针字段并带 `value` tag 时，应用启动会崩溃，而不是得到可诊断的绑定错误。
- 复现：新增 `TestInjecting/wire_error_-_embedded_pointer_value_tag`，修复前会 panic。
- 修复：仅对匿名 struct 值字段递归；其他带 `value` tag 的字段走普通 `RefreshField` 绑定路径，让不支持的指针类型返回错误。
- 验证：`go test ./gs/internal/gs_core/injecting -run 'TestInjecting/wire_error_-_embedded_pointer_value_tag'` 通过；`go test ./gs/internal/gs_core/injecting` 通过；`go test -race ./gs/internal/gs_core/injecting` 通过；`go test ./...` 通过；`go vet ./...` 通过。
- 提交：本提交 `avoid embedded pointer value tag panic`。
- 备注：与 `conf.bindStruct` 对匿名非 struct 字段的保护保持一致。
