# Bug 14：配置循环引用栈溢出

- 状态：已提交
- 位置：`conf/bind.go`
- 预期来源：运行错误；配置引用解析遇到用户配置错误时应返回可诊断错误，不能无限递归直到进程栈溢出。
- 问题：`resolveString` 和 `resolve` 递归展开 `${...}` 时没有记录当前解析路径，`a=${b}`、`b=${a}` 会无限递归。
- 影响：错误配置可能直接导致测试进程或应用进程崩溃，且无法通过普通 error 路径定位配置问题。
- 复现：直接先跑红测不安全，因为当前实现会触发栈溢出并杀掉测试进程。修复后新增 `TestProperties_Resolve/circular_reference` 覆盖该路径。
- 修复：为解析过程传递当前递归路径 map，进入某个属性 key 时记录，退出时删除；同一路径再次遇到该 key 返回 `circular property reference` 错误。独立位置重复引用同一 key 保持可用。
- 验证：`go test ./conf -run TestProperties_Resolve` 通过；`go test ./conf` 通过；`go test -race ./conf` 通过；`go test ./...` 通过；`go vet ./...` 通过。
- 提交：本提交 `detect circular config references`。
- 备注：不改变默认值解析和普通重复引用语义。
