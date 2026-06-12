# Bug 12：IndexArg 固定参数重复覆盖

- 状态：已提交
- 位置：`gs/internal/gs_arg/arg.go`
- 预期来源：API 命名与一致性行为；`IndexArg` 明确指定构造函数参数位置，`NewArgList` 已校验混用和越界，重复指定同一个固定参数也应视为无效参数列表。
- 问题：多个 `IndexArg` 指向同一个固定参数时，后一个会静默覆盖前一个。
- 影响：用户配置构造参数时可能误写重复 index，框架不会报错，最终用错误参数启动应用，排查困难。
- 复现：新增 `TestArgList_New/duplicate_fixed_argument_index`，修复前不会返回错误。
- 修复：在处理 indexed 参数时记录固定参数 index；固定参数重复时返回错误。变长参数的重复 index 保持已有追加语义不变。
- 验证：`go test ./gs/internal/gs_arg -run 'TestArgList_New/(duplicate_fixed_argument_index|variadic_success_with_indexed_args)'` 通过；`go test ./gs/internal/gs_arg` 通过；`go test -race ./gs/internal/gs_arg` 通过；`go test ./...` 通过；`go vet ./...` 通过。
- 提交：本提交 `reject duplicate fixed index args`。
- 备注：已有 `variadic success with indexed args` 覆盖了变长参数重复 index 的允许行为，本修复不改变该语义。
