# Bug 11：Refresh 插件类型不匹配 panic

- 状态：已提交
- 位置：`log_refresh.go`
- 预期来源：`RegisterPlugin` 是公开 API，允许注册任意 struct；`Refresh` 对配置错误通常返回 error，不应因用户注册了错误类型的插件而 panic。
- 问题：`Refresh` 创建 appender/logger 插件后直接执行 `v.Interface().(Appender)` 和 `v.Interface().(Logger)`；如果配置的插件 struct 不实现对应接口，会触发 panic。
- 影响：错误插件配置会导致进程崩溃，而不是从 `RefreshConfig` 返回可处理的配置错误。
- 复现：补充 `TestRefreshConfigPluginTypeMismatchReturnsError`，注册不实现 `Appender/Logger` 的插件并分别配置到 appender/logger，当前错误行为是 panic。
- 修复：`newPluginFromType` 返回插件名；appender/logger 装配处先做安全类型断言，不匹配时返回带插件名和配置节点的错误。
- 验证：`go test ./...` 通过。
- 提交：本提交 `return error for refresh plugin type mismatch`
- 备注：不改变 `PluginElement` 内部类型校验；这里只收敛顶层 appender/logger 装配。
