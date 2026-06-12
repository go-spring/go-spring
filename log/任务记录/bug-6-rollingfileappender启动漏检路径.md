# Bug 6：RollingFileAppender 启动漏检路径

- 状态：已提交
- 位置：`plugin_appender.go`
- 预期来源：`Appender` 生命周期接口要求 `Start() error` 返回启动错误；`RollingFileAppender.Start` 注释说明启动时会打开初始日志文件；`FileAppender.Start` 也会在启动阶段返回文件打开错误。
- 问题：`RollingFileAppender.Start` 只初始化 `RollingFileWriter`，没有打开目标文件；非法目录等路径错误会被延迟到首次写日志时通过 `ReportError` 暴露，`Refresh` 会误判配置启动成功。
- 影响：使用 rolling file appender/logger 时，错误的文件路径不能在刷新配置或启动阶段失败，用户可能丢失日志且无法从 `Refresh` 返回值感知配置错误。
- 复现：补充 `TestRollingFileAppender/Start_error`，使用不存在目录的绝对文件名启动 rolling appender，当前错误行为是 `Start` 返回 nil。
- 修复：`RollingFileAppender.Start` 初始化 writer 后立即执行一次 `Rotate`，让初始文件打开错误从 `Start` 返回。
- 验证：`go test ./...` 通过。
- 提交：本提交 `fix rolling file appender start error`
- 备注：修复后会在启动阶段创建带时间后缀的 rolling 日志文件，符合当前 `Start` 注释和生命周期语义。
