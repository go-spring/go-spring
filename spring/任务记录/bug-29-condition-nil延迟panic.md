# Bug 29：Condition Nil 延迟 Panic

- 状态：已提交
- 位置：`gs/internal/gs_bean/bean.go`、`gs/internal/gs_arg/arg.go`
- 预期来源：一致性行为。条件构造器已经拒绝 nil 条件，链式追加条件的 API 也应在入口处保持同样校验。
- 问题：`BeanDefinition.Condition(nil)` 和 `BindArg.Condition(nil)` 会把 nil 条件保存下来，后续解析 bean 或执行绑定参数时才触发 nil 指针 panic。
- 影响：用户传入 nil 条件时，错误延迟到容器刷新或注入阶段暴露，panic 信息不可定位到真正的调用入口。
- 复现：新增 `TestBeanDefinition/condition` 和 `TestBindArg_Condition/nil condition`。
- 修复：在两个 `Condition` 方法中复用条件校验逻辑，入口处拒绝 nil 条件。
- 验证：
  - `go test ./gs/internal/gs_bean -run 'TestBeanDefinition/condition'`：修复前失败，复现 nil 条件被静默接受；修复后通过。
  - `go test ./gs/internal/gs_arg -run 'TestBindArg_Condition/nil_condition'`：修复前失败，复现 nil 条件被静默接受；修复后通过。
  - `go test ./gs/internal/gs_cond`：通过。
  - `go test ./gs/internal/gs_arg ./gs/internal/gs_bean ./gs/internal/gs_cond ./gs/internal/gs_core/resolving ./gs/internal/gs_core/injecting`：通过。
  - `go test ./...`：通过。
- 提交：本提交 `reject nil appended conditions`。
- 备注：零参数 `Condition()` 仍保持无操作；本次只拒绝显式 nil 条件。
