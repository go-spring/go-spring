# Bug 19：Slice 索引缺口静默丢弃

- 状态：已提交
- 位置：`conf/bind.go`
- 预期来源：配置绑定应完整消费显式索引配置；`flatten.Storage.SliceEntries` 注释说明只收集 slice entries、不强制索引连续，因此绑定层需要处理缺口，不能静默成功。
- 问题：`bindSlice` 从索引 0 开始循环，遇到第一个不存在的 `key[i]` 就停止，导致只配置 `numbers[1]` 或配置 `numbers[0]`、`numbers[2]` 时后续值被静默丢弃。
- 影响：用户配置的 slice 项可能没有进入目标结构体，应用仍启动成功，造成隐蔽配置错误。
- 复现：新增 `TestSliceBinding/indexed_slice_missing_zero_index` 和 `TestSliceBinding/indexed_slice_gap`。
- 修复：在显式 indexed entries 转为临时 storage 前解析索引并校验必须从 0 连续。
- 验证：
  - `go test ./conf -run TestSliceBinding`：修复前两个缺口用例返回 nil error；修复后通过。
  - `go test ./conf ./gs/internal/gs_conf ./gs/internal/gs_core/injecting ./gs/internal/gs_dync`：通过。
  - `go test ./...`：通过。
  - `go test -race ./conf`：通过。
- 提交：本提交 `reject sparse indexed slices`。
- 备注：逗号分隔字符串生成的索引天然连续，不改变该路径。
