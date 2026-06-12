# Bug 6：Slice 注入显式顺序被覆盖

- 状态：已提交
- 位置：`gs/internal/gs_core/injecting/injecting.go`
- 预期来源：`getBeans` 注释说明 collection tags 用于 ordering and selection，并保证 `before '*' -> '*' -> after '*'`；`inject:"biz,*,sys"` 这类公开用法表达了用户指定顺序。
- 问题：`getBeans` 已按 tag 顺序组装 bean 列表，但 `autowire` 在写入 slice 前无条件按 bean name 排序，导致 `inject:"z,a"` 实际注入为 `a,z`。
- 影响：用户无法可靠控制 middleware、filter、logger 等 slice 依赖的执行顺序，可能导致运行行为错误。
- 复现：新增 `TestInjecting/slice_injection_keeps_explicit_order`，显式要求 `z,a` 顺序，现有实现会得到 `a,z`。
- 修复：只有未提供显式 tags 时才按 bean name 排序；一旦用户提供 tags，保留 `getBeans` 计算出的顺序。
- 验证：`go test ./gs/internal/gs_core/injecting -run TestInjecting/slice_injection_keeps_explicit_order` 通过；`go test ./gs/internal/gs_core/injecting` 通过；`go test ./...` 通过。
- 提交：本提交 `fix explicit slice injection order`。
- 备注：本修复不调整 wildcard 内部剩余 bean 的排序策略，只防止显式 tag 顺序被最终排序覆盖。
