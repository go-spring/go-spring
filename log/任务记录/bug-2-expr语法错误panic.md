# Bug 2：expr 语法错误 panic

- 状态：已提交
- 位置：`expr/parse.go`
- 预期来源：`Parse` 函数签名通过 `error` 返回解析失败；现有 `expr/parse_test.go` 已将多种无效语法标记为 `wantErr`，说明语法错误应作为普通错误处理。
- 问题：ANTLR 错误监听器记录语法错误后，`Parse` 仍继续 walk parse tree；部分无效输入会产生不完整节点，listener 访问空节点时 panic，最终错误里包含 `[PANIC]` 和运行时栈。
- 影响：用户配置写错时错误信息包含内部 panic 栈，掩盖真正的语法错误，也让上层 `RefreshConfig` 错误不稳定且难读。
- 复现：增强 `expr/parse_test.go`，要求所有预期解析失败的用例不能返回包含 `[PANIC]` 的错误。
- 修复：构建 root parse tree 后，如果错误监听器已记录语法错误，立即返回该错误，不再 walk 不完整语法树。
- 验证：`go test ./expr` 通过；`go test ./...` 通过。
- 提交：本提交 `fix expr syntax error handling`
- 备注：保留 `recover` 兜底处理非语法错误 panic；本修复只调整已知语法错误路径。
