# Bug 12：GetLogger 刷新前 panic

- 状态：已提交
- 位置：`log_logger.go`
- 预期来源：`GetLogger` 是公开 API，说明会获取或创建 `LoggerWrapper`；未刷新配置时普通日志 API 会使用默认 logger，命名 logger wrapper 也应有可用默认值。
- 问题：`GetLogger` 新建 wrapper 时没有初始化 `atomic.Pointer`，用户在 `Refresh` 前调用 `GetLogger(...).Write` 或 `Enable` 会 nil 指针 panic。
- 影响：使用命名 logger 的代码如果早于配置刷新写日志，会直接崩溃；`Destroy` 后 wrapper 也可能继续指向已停止 logger。
- 复现：补充 `TestGetLoggerBeforeRefreshUsesDefaultLogger`，创建命名 logger 后直接 `Write`，当前错误行为是 nil pointer panic。
- 修复：为 `LoggerWrapper` 增加 `reset`，新建 wrapper 时绑定 `defaultLogger`；`Destroy` 同时重置所有命名 wrapper。
- 验证：`go test ./...` 通过。
- 提交：本提交 `initialize logger wrappers with default logger`
- 备注：不改变 `Refresh` 成功后显式绑定命名 logger 的行为。
