# Bug 27：HTTP Server Nil Mux Panic

- 状态：已提交
- 位置：`gs/http.go`
- 预期来源：`NewSimpleHttpServer` 是公开构造器；`net/http.Server` 允许 `Handler` 为 nil 并使用默认 mux，因此 nil mux 不应导致构造阶段 panic。
- 问题：`NewSimpleHttpServer` 直接访问 `h.Handler`，当调用方传入 nil `*HttpServeMux` 时触发空指针 panic。
- 影响：用户直接调用构造器或错误注入 nil mux bean 时，应用在构造阶段崩溃。
- 复现：新增 `TestNewSimpleHttpServer/nil_mux`。
- 修复：构造器先判断 mux 是否为 nil，nil 时保留 `http.Server.Handler` 为 nil。
- 验证：
  - `go test ./gs -run TestNewSimpleHttpServer`：修复前复现 nil mux 空指针 panic；修复后通过。
  - `go test ./gs ./gs/internal/gs_app ./gs/internal/gs_core/injecting`：通过。
  - `go test ./...`：通过。
  - `go test -race ./gs`：通过。
- 提交：本提交 `allow nil HTTP serve mux`。
- 备注：非 nil mux 的行为保持不变。
