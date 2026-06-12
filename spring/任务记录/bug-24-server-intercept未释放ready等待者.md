# Bug 24：Server Intercept 未释放 Ready 等待者

- 状态：已提交
- 位置：`gs/internal/gs_app/app.go`
- 预期来源：应用启动失败进入 server intercept 后，应能继续执行 shutdown 并等待所有已启动 server 退出；已有 Bug 8 要求 shutdown 等待 server stop 完成。
- 问题：多个 server 启动时，某个 server 返回错误会触发 `sig.Intercept()`，但 `Start` 在 `sig.Intercepted()` 错误路径直接返回，没有关闭 readiness channel。已经调用 `TriggerAndWait()` 的其他 server 会继续阻塞，后续 `WaitForShutdown` 卡在 `app.wg.Wait()`。
- 影响：部分 server 启动失败时，应用 shutdown 可能无法完成，测试或进程挂起。
- 复现：新增 `TestApp/server_intercept_releases_ready_waiters`，构造一个已等待 readiness 的 server 和一个返回错误的 server。
- 修复：`Start` 在所有 server ready/intercept 后先关闭 readiness channel，再根据 intercept 状态返回错误或成功。
- 验证：
  - `go test ./gs/internal/gs_app -run 'TestApp/server_intercept_releases_ready_waiters'`：修复前 `WaitForShutdown` 超时；修复后通过。
  - `go test ./gs/internal/gs_app ./gs ./gs/internal/gs_core/injecting ./gs/internal/gs_core/resolving`：通过。
  - `go test ./...`：通过。
  - `go test -race ./gs/internal/gs_app`：通过。
- 提交：本提交 `release ready waiters on server intercept`。
- 备注：不改变单 server 错误返回的错误语义，只释放已经等待的 server goroutine。
