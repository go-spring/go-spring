# Bug 34：Dync Refresh Nil Storage

- 状态：已提交
- 位置：`gs/internal/gs_dync/dync.go`
- 预期来源：一致性行为。配置绑定公开入口已经拒绝 nil storage，动态刷新入口也不应接受 nil 配置源。
- 问题：`Properties.Refresh(nil)` 在没有已注册刷新对象时会把内部 storage 设置为 nil 并返回成功；有刷新对象时才在后续绑定阶段报错。
- 影响：运行时刷新可能把配置管理器置为 nil storage 状态，后续读取或字段绑定再出现错误，且失败点远离真正的刷新调用。
- 复现：新增 `TestDync/refresh nil storage`。
- 修复：`Refresh` 入口先校验 nil storage，失败时直接返回错误且不修改现有配置。
- 验证：
  - `go test ./gs/internal/gs_dync -run 'TestDync/refresh_nil_storage'`：修复前失败，复现 nil storage 被写入；修复后通过。
  - `go test ./gs/internal/gs_dync`：通过。
  - `go test ./...`：通过。
- 提交：本提交 `reject nil dync refresh storage`。
- 备注：不改变非 nil 空配置刷新行为。
