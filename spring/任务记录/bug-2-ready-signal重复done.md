# Bug 2：ReadySignal 重复 Done

- 状态：已提交
- 位置：`gs/internal/gs_app/signal.go`
- 预期来源：`Server` 生命周期注释说明 Server ready 后仍可能返回错误并触发应用 shutdown；`ReadySignal` 应协调 ready 和异常拦截，不应因同一 Server 先 ready 后出错而 panic。
- 问题：`TriggerAndWait()` 和 `Intercept()` 都会调用 `WaitGroup.Done()`。当同一个 Server 先调用 `TriggerAndWait()`，后续 `Run()` 返回 error 时，应用 goroutine 会调用 `Intercept()`，导致同一个计数被 Done 两次。
- 影响：运行中的 Server 在 ready 后返回错误时，应用可能触发 `sync: negative WaitGroup counter` panic，而不是按预期记录错误并执行 shutdown。
- 复现：新增 `TestReadySignal/intercept_after_ready`，先 `Add()`、`TriggerAndWait()`，再调用 `Intercept()`，现有实现会 panic。
- 修复：为 `ReadySignalImpl` 增加受互斥锁保护的 pending 计数，`TriggerAndWait()` 和 `Intercept()` 共享同一个释放路径，确保每次 `Add()` 最多释放一次 WaitGroup 计数。
- 验证：`go test ./gs/internal/gs_app -run TestReadySignal` 通过；`go test ./gs/internal/gs_app` 通过；`go test ./...` 通过。
- 提交：本提交 `fix ready signal repeated done`。
- 备注：本修复只保证计数释放不会重复，不改变 ready channel 的关闭语义。
