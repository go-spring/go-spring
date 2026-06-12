# Bug 9：插件切片索引缺口静默忽略

- 状态：已提交
- 位置：`plugin.go`
- 预期来源：`PluginElement` 切片配置使用 `name[index]` 表示数组元素；已有简单数组属性遇到缺少 `0` 时不会成功注入空结果，插件切片也不应静默忽略已配置元素。
- 问题：`injectArrayElement` 发现 indexed 配置后从 `[0]` 连续遍历；如果用户只配置 `layout[1]` 或中间缺少索引，循环会提前结束并返回成功，后续元素被忽略。
- 影响：用户配置的 appender/layout/logger 子元素可能完全丢失或部分丢失，刷新配置仍返回成功，实际日志链路与配置不一致。
- 复现：补充 `TestInjectElement/missing_indexed_element_-_slice_interface`，只配置 `test.layout[1].type=TextLayout` 时当前错误行为是 `newPlugin` 成功且得到空切片。
- 修复：在注入 indexed 插件切片前解析所有出现的索引，要求从 `0` 开始连续；缺口或非法索引直接返回错误。
- 验证：`go test ./...` 通过。
- 提交：本提交 `reject sparse plugin element indexes`
- 备注：不改变连续索引和单元素配置路径。
