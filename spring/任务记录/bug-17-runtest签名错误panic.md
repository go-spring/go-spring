# Bug 17：RunTest 签名错误 Panic

- 状态：已提交
- 位置：`gs/app.go`
- 预期来源：公开注释要求 `RunTest` 的测试函数签名为 `func(*TestStruct)`，公开测试入口应报告清晰错误，而不是由 `reflect` 边界直接 panic。
- 问题：`RunTest` 未校验 `f` 的类型、入参数量、入参形态和 nil 函数，直接调用 `ft.In(0).Elem()` 与 `reflect.Value.Call`，错误签名会触发反射 panic。
- 影响：用户传错测试函数签名时，测试进程得到底层反射 panic，错误信息不指向 `RunTest` 的签名约束。
- 复现：新增 `TestValidateRunTestFunc` 覆盖 nil、非函数、nil 函数、参数数量错误、非指针参数和非结构体指针参数。
- 修复：为 `RunTest` 增加签名验证辅助函数，入口在启动应用前用 `t.Fatal` 报告明确错误。
- 验证：
  - `go test ./gs`：先因缺少 `validateRunTestFunc` 失败，用于固定新增复现测试；修复后通过。
  - `go test ./...`：通过。
  - `go test -race ./gs`：通过。
- 提交：本提交 `validate RunTest function signature`。
- 备注：`RunTest` 的 `*testing.T` 失败语义不适合在同进程中直接断言 `t.Fatal`，因此验证集中在签名验证辅助函数和现有 `RunTest` 正常路径。
