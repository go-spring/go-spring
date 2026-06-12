# Bug 5：默认 root 刷新后 layout 丢失

- 状态：已提交
- 位置：`log.go`
- 预期来源：`RootLoggerName` 注释说明 root logger 是未匹配日志的默认 logger；`Refresh` 中未配置 `logger.root` 时会保留 `defaultLogger`，说明默认 root 应在刷新后继续可用。
- 问题：`defaultLogger` 只预置了内部 `appender` 的 layout，没有设置 `ConsoleLogger.Layout` 字段；`Refresh` 启动默认 root 时调用 `ConsoleLogger.Start`，会用 nil `Layout` 重建 appender，之后默认标签写日志会因 nil layout panic。
- 影响：用户刷新了不含 `logger.root` 的配置后，未匹配标签或默认标签日志可能直接 panic。
- 复现：补充 `TestRefreshConfigWithoutRootKeepsDefaultLoggerLayout`，刷新空配置后写默认标签日志应正常输出。
- 修复：为 `defaultLogger` 的 `ConsoleLogger.Layout` 字段设置同一个默认 TextLayout，使 Start 后仍有可用 layout。
- 验证：`go test ./...` 通过。
- 提交：本提交 `fix default root logger layout after refresh`
- 备注：不改变显式配置 `logger.root` 时的行为。
