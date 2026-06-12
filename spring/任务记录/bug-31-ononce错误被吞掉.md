# Bug 31：OnOnce 错误被吞掉

- 状态：已提交
- 位置：`gs/gs.go`
- 预期来源：API 命名和注释。`OnOnce` 承诺条件只求值一次，后续调用返回缓存结果；首次求值返回的错误也属于结果的一部分，不应在后续调用中变成 nil。
- 问题：`OnOnce` 只缓存 bool 结果，不缓存 error。第一次 `Matches` 返回错误后，第二次调用会返回 `false, nil`。
- 影响：条件错误可能在重复匹配路径中被隐藏，导致容器或调用方误认为只是条件不满足，而不是配置/依赖查询失败。
- 复现：新增 `TestOnOnce/caches error`。
- 修复：在 `OnOnce` 闭包内同时缓存首次匹配的 bool 和 error。
- 验证：
  - `go test ./gs -run 'TestOnOnce/caches_error'`：修复前失败，复现第二次调用吞掉首次错误；修复后通过。
  - `go test ./gs -run 'TestOnOnce'`：通过。
  - `go test ./...`：通过。
- 提交：本提交 `cache OnOnce match errors`。
- 备注：不改变条件只执行一次的行为。
