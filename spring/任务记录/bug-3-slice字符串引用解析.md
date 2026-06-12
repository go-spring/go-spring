# Bug 3：Slice 字符串引用解析

- 状态：已提交
- 位置：`conf/bind.go`
- 预期来源：`conf.Bind` 的字符串绑定和 `conf.Resolve` 都支持 `${key}` 引用；slice 绑定也支持逗号分隔字符串，因此同一个配置值中的引用应在拆分前按一致规则解析。
- 问题：`getSlice` 对逗号分隔字符串没有先基于原始配置执行 `resolveString`，而是直接拆分后放入临时 storage。后续元素绑定只看到临时 storage，导致 `${a}`、`${b}` 这类引用无法访问原始配置。
- 影响：用户配置 `numbers=${a},${b}` 或在 slice 默认值中使用 `${...}` 引用时，绑定会错误地报告引用属性不存在。
- 复现：新增 `TestSliceBinding/comma_separated_string_resolves_references`，绑定 `numbers=${a},${b}` 到 `[]int`，现有实现返回 `property "a" does not exist`。
- 修复：对单值逗号分隔的 slice 来源先用原始 storage 解析引用，再拆分为元素。
- 验证：`go test ./conf -run TestSliceBinding/comma_separated_string_resolves_references` 通过；`go test ./conf` 通过；`go test ./...` 通过。
- 提交：本提交 `fix slice string reference binding`。
- 备注：不改变显式索引形式 `list[0]` 的既有解析逻辑。
