# Bug 9：gs_app Race 测试不安全

- 状态：已提交
- 位置：`gs/internal/gs_app/app_test.go`
- 预期来源：运行错误；`go test -race` 应能在核心包测试中通过，测试代码本身不能引入数据竞争。
- 问题：`app_test.go` 将全局日志输出指向 `bytes.Buffer`，应用启动和关闭 goroutine 会并发写入该 buffer，测试主 goroutine 还会读取或重置它；同时 `success` 子测试在额外 goroutine 中读取 `app.Servers` 和 `app.Runners`，与 `Start` 注入字段并发。
- 影响：`go test -race ./gs/internal/gs_app` 稳定报告 data race，使生命周期相关代码无法用 race detector 验证，也可能掩盖真实并发问题。
- 复现：执行 `go test -race ./gs/internal/gs_app`，race detector 报告 `bytes.Buffer` 并发读写、`log.Stdout` 重设与日志写入并发，以及 `app.Servers` 字段读写竞争。该命令本身就是复现用例，因此未额外添加失败测试。
- 修复：为测试日志引入带锁 buffer，`log.Stdout` 在初始化时固定到该 writer，测试间只清空 buffer；将 `success` 子测试对 `app.Servers` 和 `app.Runners` 的断言移回主测试 goroutine。
- 验证：`go test ./gs/internal/gs_app` 通过；`go test -race ./gs/internal/gs_app` 通过；`go test ./...` 通过。
- 提交：本提交 `fix gs app race-safe tests`。
- 备注：仅修复测试自身的并发不安全点，不改变 `App` 生命周期行为。
