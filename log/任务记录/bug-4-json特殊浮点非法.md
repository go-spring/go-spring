# Bug 4：JSON 特殊浮点非法

- 状态：已提交
- 位置：`field_encoder.go`
- 预期来源：`JSONLayout` 和 `JSONEncoder` 的公开语义是输出 JSON；JSON 标准不允许裸 `NaN`、`+Inf`、`-Inf` 数值。
- 问题：`JSONEncoder.AppendFloat64` 直接使用 `strconv.FormatFloat` 写入结果，遇到 `math.NaN()` 或无穷大时会输出 `NaN`、`+Inf`、`-Inf`，导致整条 JSON 日志不可解析。
- 影响：用户记录特殊浮点值时，JSON 日志管道、采集器或下游 `json.Unmarshal` 会解析失败。
- 复现：补充 JSON 编码测试，要求特殊浮点输出仍然是合法 JSON。
- 修复：JSON 编码中将非有限浮点值写为 JSON 字符串，有限浮点仍按数字输出。
- 验证：`go test ./...` 通过。
- 提交：本提交 `fix JSON encoding for special floats`
- 备注：TextEncoder 顶层文本输出仍保持原有文本表示；只修复 JSON 编码语义。
