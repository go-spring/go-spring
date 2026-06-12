# Bug 32：Module Group Nil 函数延迟 Panic

- 状态：已提交
- 位置：`gs/gs.go`
- 预期来源：公开 API 的一致性行为。`Provide`、`BindArg`、生命周期函数等入口已经拒绝 nil 函数，模块和分组注册函数也应在注册阶段校验。
- 问题：`Module(nil, nil)` 和 `Group("${items}", nil, nil)` 会接受 nil 函数，把错误延迟到应用启动或模块执行阶段。
- 影响：用户在 init 阶段写错注册函数时，错误位置会偏移到启动流程，可能表现为 nil 函数调用或构造器 nil panic。
- 复现：新增 `TestModuleNilFunction` 和 `TestGroupNilFunction`。
- 修复：在 `Module` 与 `Group` 入口处拒绝 nil 函数。
- 验证：
  - `go test ./gs -run 'TestModuleNilFunction'`：修复前失败，复现 nil module 函数未拒绝；修复后通过。
  - `go test ./gs -run 'TestGroupNilFunction'`：修复前失败，复现 nil group 构造函数未拒绝；修复后通过。
  - `go test ./gs`：通过。
  - `go test ./...`：通过。
- 提交：本提交 `reject nil module group functions`。
- 备注：`Group` 的 destroy 函数 nil 仍表示不注册销毁回调，保持允许。
