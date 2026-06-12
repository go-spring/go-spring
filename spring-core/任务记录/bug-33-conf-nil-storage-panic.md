# Bug 33：Conf Nil Storage Panic

- 状态：已提交
- 位置：`conf/conf.go`、`conf/bind.go`
- 预期来源：公开 API 的错误处理约定。`conf.Bind`、`conf.BindValue`、`conf.Resolve` 都返回 error，非法输入应返回可定位错误，而不是 panic。
- 问题：传入 nil `flatten.Storage` 时，内部直接调用 `Value`、`Exists`、`MapKeys` 或 `SliceEntries`，触发 nil 接口方法调用 panic。
- 影响：调用方配置来源初始化失败并传入 nil storage 时，绑定/解析流程会崩溃，无法通过 error 分支处理。
- 复现：新增 `TestProperties_Bind/nil storage`、`TestProperties_Bind/bind value nil storage`、`TestProperties_Resolve/nil storage`。
- 修复：在公开入口处统一检测 nil storage，并返回 `properties storage cannot be nil` 错误。
- 验证：
  - `go test ./conf -run 'TestProperties_Bind/nil_storage'`：修复前 panic；修复后通过。
  - `go test ./conf -run 'TestProperties_Bind/bind_value_nil_storage'`：修复前 panic；修复后通过。
  - `go test ./conf -run 'TestProperties_Resolve/nil_storage'`：修复前 panic；修复后通过。
  - `go test ./conf`：通过。
  - `go test ./...`：通过。
- 提交：本提交 `reject nil conf storage`。
- 备注：不改变非 nil 空配置的现有行为。
