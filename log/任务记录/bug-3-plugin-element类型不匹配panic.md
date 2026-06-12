# Bug 3：PluginElement 类型不匹配 panic

- 状态：已提交
- 位置：`plugin.go`
- 预期来源：插件注入函数通过 `error` 返回配置错误；已有 `TestInjectElement` 覆盖插件不存在和注入失败时返回错误，不应因用户配置了错误插件类型而 panic。
- 问题：`PluginElement` 注入接口字段或接口切片时，只按插件名创建实例，没有检查创建出来的插件是否实现目标接口；例如 `Layout` 字段配置为 `FileAppender` 会在 `reflect.Set` 或 `reflect.Append` 时 panic。
- 影响：用户配置中引用了存在但类别错误的插件时，刷新配置可能直接 panic，无法得到可诊断的配置错误。
- 复现：补充 `TestInjectElement` 中单个接口字段和接口切片的类型不匹配用例。
- 修复：`createPlugin` 在创建接口插件前校验 `*PluginClass` 是否可赋值给目标接口，类型不匹配时返回错误。
- 验证：`go test ./...` 通过。
- 提交：本提交 `fix plugin element type validation`
- 备注：只校验接口注入的插件类型，不改变结构体指针元素的既有注入规则。
