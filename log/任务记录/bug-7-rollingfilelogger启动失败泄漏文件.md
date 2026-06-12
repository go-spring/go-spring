# Bug 7：RollingFileLogger 启动失败泄漏文件

- 状态：已提交
- 位置：`plugin_logger.go`
- 预期来源：`Lifecycle.Start() error` 的错误路径不应遗留已启动资源；`Refresh` 失败清理只会停止已经成功启动并登记的 logger/appender。
- 问题：`RollingFileLogger.Start` 先启动内部 `RollingFileAppender`，再启动内部 logger；如果内部 logger 启动失败，例如异步模式 `bufferSize` 太小，已经打开的 rolling file appender 不会被停止。
- 影响：刷新或启动 rolling file logger 失败时可能泄漏文件句柄，并在 `fileManager` 中残留引用。
- 复现：补充 `TestRollingFileLoggerStartErrorCleansAppenders`，构造异步 rolling file logger 且 `BufferSize=10`，启动失败后检查临时日志文件没有残留打开引用。
- 修复：记录已成功启动的内部 appender；任一 appender 启动失败或内部 logger 启动失败时，反向停止已经启动的 appender。
- 验证：`go test ./...` 通过。
- 提交：本提交 `fix rolling file logger start cleanup`
- 备注：不改变成功启动路径；只收敛失败路径资源清理。
