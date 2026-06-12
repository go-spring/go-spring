# Bug 10：Cleanup 回调并发写 Map

- 状态：已提交
- 位置：`gs/internal/gs_core/injecting/injecting_test.go`
- 预期来源：运行错误；`go test -race` 应能在核心包测试中通过，测试代码不能在 runtime cleanup 回调中制造数据竞争。
- 问题：`TestDyncValue/without_dync_value` 注册两个 `runtime.AddCleanup` 回调，并在回调 goroutine 中并发写入同一个 `map[string]struct{}`。
- 影响：`go test -race ./...` 稳定报告 data race，导致注入模块无法用 race detector 做全量验证。
- 复现：执行 `go test -race ./...`，race detector 报告 `injecting_test.go` 中两个 cleanup 回调并发访问 `release` map。该命令本身就是复现用例，因此未额外添加失败测试。
- 修复：将 cleanup 回调的释放信号写入有缓冲 channel，由测试主 goroutine 汇总为 map 后断言，避免多个 cleanup goroutine 共享写 map。
- 验证：`go test ./gs/internal/gs_core/injecting` 通过；`go test -race ./gs/internal/gs_core/injecting` 通过；`go test ./...` 通过；`go test -race ./...` 通过。
- 提交：本提交 `fix cleanup race in injecting test`。
- 备注：只修复测试同步方式，不改变动态值或注入器行为。
