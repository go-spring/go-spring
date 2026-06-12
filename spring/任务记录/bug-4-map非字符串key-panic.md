# Bug 4：Map 非字符串 Key Panic

- 状态：已提交
- 位置：`conf/bind.go`
- 预期来源：`conf.Bind` 是公开 API，绑定失败应返回 error；`typeutil.IsPropBindingTarget` 只按 map 元素类型判断目标是否合法，因此 `map[int]string` 会进入 map 绑定路径。
- 问题：`bindMap` 收集配置 key 后固定使用 `reflect.ValueOf(key)` 写入目标 map。目标 map key 不是 string 时，`SetMapIndex` 会因 key 类型不匹配 panic。
- 影响：用户误将配置绑定到 `map[int]string`、`map[uint]int` 等非字符串 key map 时，应用会 panic，而不是得到可处理的绑定错误。
- 复现：新增 `TestMapBinding/non_string_key_returns_error`，绑定 `map[int]string`，现有实现会 panic。
- 修复：`bindMap` 明确拒绝非 string key 的 map，并返回错误。
- 验证：`go test ./conf -run TestMapBinding/non_string_key_returns_error` 通过；`go test ./conf` 通过；`go test ./...` 通过。
- 提交：本提交 `fix map binding non-string key panic`。
- 备注：本修复不实现非字符串 key 的解析转换，只防止公开 API panic。
