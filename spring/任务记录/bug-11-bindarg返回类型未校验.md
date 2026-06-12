# Bug 11：BindArg 返回类型未校验

- 状态：已提交
- 位置：`gs/internal/gs_arg/arg.go`
- 预期来源：一致性行为；`ValueArg.GetArgValue` 会校验返回值是否可赋给目标参数类型，`BindArg.GetArgValue(ctx, t)` 也接收了目标类型 `t`，应在返回前执行同样校验。
- 问题：`BindArg.GetArgValue` 执行绑定函数后直接返回 `out[0]`，不检查其类型是否可赋给目标参数类型。
- 影响：绑定函数返回类型与构造函数参数不匹配时，上层 `reflect.Call` 可能 panic，而不是在参数解析阶段返回可诊断错误。
- 复现：新增 `TestBindArg_GetArgValue/return_value_type_mismatch`，修复前不会返回错误。
- 修复：在绑定函数成功返回后，复用 `ValueArg` 一致的 assignability 校验；若返回值不能赋给目标类型，返回错误。
- 验证：`go test ./gs/internal/gs_arg -run 'TestBindArg_GetArgValue/return_value_type_mismatch'` 通过；`go test ./gs/internal/gs_arg` 通过；`go test -race ./gs/internal/gs_arg` 通过；`go test ./...` 通过；`go vet ./...` 通过。
- 提交：本提交 `fix bind arg return type validation`。
- 备注：不改变条件不满足时返回 invalid value 的行为，也不改变绑定函数自身错误优先级。
