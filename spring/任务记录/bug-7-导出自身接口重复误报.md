# Bug 7：导出自身接口重复误报

- 状态：已提交
- 位置：`gs/internal/gs_bean/bean.go`
- 预期来源：`Export` 用于注册 bean 额外导出的接口；如果构造函数本身已返回接口类型，再导出同一个接口应当是幂等操作，而不是制造重复 bean。
- 问题：`BeanDefinition.Export` 只去重 `exports` 列表，没有过滤与 `GetType()` 相同的接口。后续 `checkDuplicateBeans` 会遍历 `append(exports, type)`，把同一个 bean 的同一接口登记两次并误报 duplicate。
- 影响：用户对返回接口类型的构造函数调用 `.Export(gs.As[Interface]())` 会导致容器启动失败，即使实际只有一个 bean。
- 复现：新增 `TestResolving/export_same_as_interface_bean_type`，构造函数返回 `Logger` 并导出 `Logger`，现有实现报 `found duplicate beans`。
- 修复：`Export` 遇到与 bean 自身类型相同的接口时直接跳过。
- 验证：`go test ./gs/internal/gs_core/resolving -run TestResolving/export_same_as_interface_bean_type` 通过；`go test ./gs/internal/gs_bean ./gs/internal/gs_core/resolving` 通过；`go test ./...` 通过。
- 提交：本提交 `fix duplicate export of own interface type`。
- 备注：保留不同接口 export 的重复检查和多 bean duplicate 检查。
