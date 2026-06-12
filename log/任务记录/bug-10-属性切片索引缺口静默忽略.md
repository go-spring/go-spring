# Bug 10：属性切片索引缺口静默忽略

- 状态：已提交
- 位置：`plugin.go`
- 预期来源：`PluginAttribute` 切片配置使用 `name[index]` 表示数组元素；出现 indexed 配置时，应完整注入用户配置的连续数组，而不是忽略后续元素。
- 问题：`getArrayValues` 从 `[0]` 开始读取，遇到第一个缺失索引就停止；如果配置了 `numbers[0]` 和 `numbers[2]`，`numbers[2]` 会被静默忽略并返回成功。
- 影响：日志插件的数组属性可能被部分注入，用户配置的标签、引用或其他切片值与运行时实际值不一致。
- 复现：补充 `TestInjectAttribute/slice_from_indexed_keys_with_missing_element`，配置 `test.numbers[0]` 和 `test.numbers[2]` 时当前错误行为是成功注入 `[]int{10}`。
- 修复：在读取 indexed 属性切片前收集所有出现的索引，要求从 `0` 开始连续；缺口或非法索引直接返回错误。插件元素切片复用同一索引校验逻辑。
- 验证：`go test ./...` 通过。
- 提交：本提交 `reject sparse attribute indexes`
- 备注：不改变逗号分隔值和连续 indexed 值的注入行为。
