# Bug 8：expr 内部重复键漏检

- 状态：已提交
- 位置：`expr/parse.go`
- 预期来源：`parseExpr` 已对展开后的配置 key 做重复检测；同一个表达式内部生成的是同一类 flat key，也应保持一致行为，避免配置被静默覆盖。
- 问题：`expr.Parse` 在解析 `Logger { level = "info", level = "debug" }` 或嵌套表达式与点号字段生成同一 key 时，会直接覆盖前一个值，不返回错误。
- 影响：用户使用 `!` 表达式配置插件时，重复字段会被后者静默覆盖，可能导致日志级别、appender 或 layout 配置与预期不一致。
- 复现：补充 `TestParse/duplicate_field` 和 `TestParse/duplicate_nested_field`，当前错误行为是 `Parse` 返回 nil error。
- 修复：为 `ParseTreeListener` 增加统一的 `setValue` 写入路径，写入前检查 key 是否已存在；解析完成后返回监听器记录的重复 key 错误。
- 验证：`go test ./...` 通过。
- 提交：本提交 `reject duplicate expr keys`
- 备注：不改变合法表达式的解析结果。
