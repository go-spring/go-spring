# Bug 1：严格解析配置标签

- 状态：已提交
- 位置：`conf/bind.go`
- 预期来源：`ParseTag` 注释说明配置标签遵循 `${key:=default}` 格式，且解析逻辑是 strict，malformed tags 应返回 invalid syntax；已有 `TestParseTag` 也验证缺少 `${` 或 `}` 会报错。
- 问题：`ParseTag` 使用 `strings.Index(tag, "${")` 和 `strings.LastIndex(tag, "}")` 截取内容，导致 `prefix${a}`、`${a}suffix`、`${a}${b}` 这类带额外内容的 tag 被当成有效标签解析。
- 影响：用户在结构体 `value` tag 中拼错或拼接了多余内容时，绑定逻辑不会及时报错，可能静默绑定到错误配置 key 或忽略错误后缀。
- 复现：新增 `TestParseTag/extra_content_outside_tag`，期望上述 malformed tags 返回错误。
- 修复：要求 `ParseTag` 只接受以 `${` 开头且以 `}` 结尾的完整标签，并拒绝 `${a}${b}` 这类同级拼接的额外 tag 内容；保留默认值中 `${...}` 引用和 `{}` 内容的既有能力。
- 验证：`go test ./conf` 通过；`go test ./...` 通过。
- 提交：本提交 `fix strict config tag parsing`。
- 备注：本问题只修复 tag 语法边界，不改变 `${key:=default}` 内部内容解析规则。
