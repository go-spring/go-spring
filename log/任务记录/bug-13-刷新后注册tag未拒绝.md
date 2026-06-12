# Bug 13：刷新后注册 tag 未拒绝

- 状态：已提交
- 位置：`log_tag.go`
- 预期来源：`RegisterTag` 注释明确要求该函数只能在初始化阶段调用，日志系统刷新后再调用应 panic；刷新时 tag 到 logger 的匹配表已经构建完成。
- 问题：`RegisterTag` 没有检查刷新状态，刷新后新注册的 tag 会留在未绑定状态，后续日志可能走默认 logger，而不是已配置的通配 tag logger。
- 影响：用户在刷新配置后动态注册 tag 时，日志路由与配置不一致，且没有显式错误提示。
- 复现：补充 `TestRegisterTagAfterRefreshPanics`，`RefreshConfig(nil)` 后调用 `RegisterTag("_app_after_refresh")`，当前错误行为是不 panic 且污染 tagRegistry。
- 修复：在全局状态中记录 `refreshed`；`Refresh` 成功后置位，`Destroy` 清除；`RegisterTag` 在刷新后调用时 panic。
- 验证：`go test ./...` 通过。
- 提交：本提交 `reject tag registration after refresh`
- 备注：`Destroy` 后允许重新注册，符合其重置全局状态的语义。
