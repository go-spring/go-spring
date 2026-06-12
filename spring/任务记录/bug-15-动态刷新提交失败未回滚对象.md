# Bug 15：动态刷新提交失败未回滚对象

- 状态：已提交
- 位置：`gs/internal/gs_dync/dync.go`
- 预期来源：代码注释和 API 行为；`Properties.Refresh` 明确说明使用两阶段刷新，失败时旧配置保留且对象不应部分更新。
- 问题：`Refresh` 在 commit 阶段逐个调用 refreshable。若前面的对象已经 commit 成新值，后面的对象返回错误，方法只恢复 `p.prop`，不会把已经 commit 的对象刷回旧值。
- 影响：动态刷新失败后，对象值可能已经部分更新，而 `Properties.Data()` 指向旧配置，运行时状态和配置源不一致。
- 复现：新增 `TestDync/commit_error_rolls_back_committed_objects`，修复前会让第一个对象保持新值 `2`。
- 修复：commit 阶段失败时先恢复旧属性源，再对所有 refreshable 执行一次 commit 刷回旧配置；若回滚也报错，则把原始 commit 错误和回滚错误一起返回。
- 验证：`go test ./gs/internal/gs_dync -run 'TestDync/commit_error_rolls_back_committed_objects'` 通过；`go test ./gs/internal/gs_dync` 通过；`go test -race ./gs/internal/gs_dync` 通过；`go test ./...` 通过；`go vet ./...` 通过。
- 提交：本提交 `rollback dynamic refresh commit failure`。
- 备注：不改变预刷新阶段失败的行为。
