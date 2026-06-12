# Bug 5：Bean Map 非字符串 Key Panic

- 状态：已提交
- 位置：`gs/internal/gs_core/injecting/injecting.go`
- 预期来源：`autowire`/`inject` 是核心依赖注入路径，注入失败应返回 error；`typeutil.IsBeanInjectionTarget` 对 map 只检查元素类型，因此 `map[int]Logger` 会进入 map 注入路径。
- 问题：map 注入时固定以 bean name 的 string 值作为 key 调用 `SetMapIndex`。目标 map key 不是 string 时，反射会 panic。
- 影响：用户误将 bean 集合注入到非字符串 key 的 map 时，应用启动会 panic，而不是得到可诊断的注入错误。
- 复现：新增 `TestWireBeanValue/map_injection_non_string_key_returns_error`，注入 `map[int]CtxLogger`，现有实现会 panic。
- 修复：map bean 注入明确拒绝非 string key，并返回错误。
- 验证：`go test ./gs/internal/gs_core/injecting -run TestInjecting/map_injection_non_string_key_returns_error` 通过；`go test ./gs/internal/gs_core/injecting` 通过；`go test ./...` 通过。
- 提交：本提交 `fix bean map injection non-string key panic`。
- 备注：本修复不实现 bean name 到非字符串 key 的转换。
