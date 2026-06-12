# Bug 18：Bind 非法 Target Panic

- 状态：已提交
- 位置：`conf/conf.go`
- 预期来源：公开注释说明 `conf.Bind` 的 target 可以是指针或 `reflect.Value`，错误 target 应返回可诊断错误；同文件已有非指针、指针指针测试也按错误返回处理。
- 问题：`Bind` 没有校验 nil 指针、无效 `reflect.Value` 和不可设置的 `reflect.Value`，随后调用 `Elem`、`Type` 或 `Set*` 时会触发反射 panic。
- 影响：调用方传入非法 target 时，配置绑定过程可能直接崩溃，而不是返回普通 error。
- 复现：新增 `TestProperties_Bind/nil_pointer_target`、`invalid_reflect_value_target`、`unsettable_reflect_value_target`。
- 修复：在进入 `BindValue` 前统一校验 target 是否为非 nil 指针，或有效且可设置的 `reflect.Value`。
- 验证：
  - `go test ./conf -run TestProperties_Bind`：修复前在 `nil_pointer_target` 复现反射 panic；修复后通过。
  - `go test ./conf ./gs/internal/gs_conf ./gs/internal/gs_core/injecting ./gs/internal/gs_dync`：通过。
  - `go test ./...`：通过。
  - `go test -race ./conf`：通过。
- 提交：本提交 `validate conf bind target`。
- 备注：该修复只补充入口校验，不改变已存在的指针指针 target 错误语义。
