# Bug 20：配置注册 Nil 处理函数

- 状态：已提交
- 位置：`conf/conf.go`、`conf/expr.go`、`conf/provider/provider.go`、`conf/reader/reader.go`
- 预期来源：配置注册入口已经对空名称和重复注册立即 panic；nil 处理函数同样属于非法注册输入，应在注册阶段失败，而不是延迟到配置读取、绑定或校验时触发 nil function panic。
- 问题：`RegisterConverter`、`RegisterValidateFunc`、`provider.Register`、`reader.Register` 接受 nil 函数并写入全局注册表。
- 影响：应用可以启动到后续使用阶段才因 nil 函数调用崩溃，错误位置远离实际注册错误。
- 复现：新增 `TestRegisterNilConverter`、`TestRegisterNilValidateFunc`、`TestRegisterNilProvider`、`TestRegisterNilReader`。
- 修复：各注册入口在写入 map 前校验函数是否为 nil，并给出明确 panic。
- 验证：
  - `go test ./conf ./conf/provider ./conf/reader`：修复前四个 nil 注册用例均因未 panic 失败；修复后通过。
  - `go test ./...`：通过。
  - `go test -race ./conf ./conf/provider ./conf/reader`：通过。
- 提交：本提交 `reject nil config registrars`。
- 备注：保留现有空名称、重复注册的 panic 语义。
