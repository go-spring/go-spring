# Bug 8：Shutdown 未等待 Server Stop

- 状态：已提交
- 位置：`gs/internal/gs_app/app.go`
- 预期来源：`WaitForShutdown` 注释说明 shutdown 后会停止所有 server，并等待 shutdown complete；方法名也表示调用返回时 shutdown 已完成。
- 问题：`WaitForShutdown` 为每个 `Server.Stop()` 启动 goroutine，但只等待 `app.wg` 中的 `Run()` goroutine，不等待 `Stop()` goroutine。若 `Run()` 已返回而 `Stop()` 仍在执行，`WaitForShutdown` 会提前关闭容器和日志并返回。
- 影响：调用者可能在 server 还没完成优雅停止时继续清理资源或断言状态；测试中也会出现 shutdown 日志和断言读取并发的 race。
- 复现：新增 `TestApp/wait_for_shutdown_waits_for_stop`，让 `Stop()` 阻塞，现有实现会在释放 `Stop()` 前返回。
- 修复：在 `WaitForShutdown` 内增加局部 `stopWg`，等待所有 `Stop()` goroutine 完成后再等待 server `Run()` goroutine 和关闭容器。
- 验证：`go test ./gs/internal/gs_app -run TestApp/wait_for_shutdown_waits_for_stop` 通过；`go test ./gs/internal/gs_app` 通过；`go test ./...` 通过。
- 提交：本提交 `fix shutdown waiting for server stop`。
- 备注：不改变 `Stop()` 的并发调用方式，只补齐等待语义。
