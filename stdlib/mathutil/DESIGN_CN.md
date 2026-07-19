# mathutil 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖的 `stdlib` 层。`mathutil` 目前只承担 JSON / 表单绑定需要的
溢出检查。

## 1. 职责与边界

- 回答"这个 `int64` / `uint64` / `float64` 能否装进 `T`？"，避免每个调用点
  都手写 `math` 常量和类型分支。
- 不是通用数值库。饱和转换、四舍五入等能力不在这里；调用方拿到 bool 后
  自行决定错误处理。

## 2. 设计说明

- 通过对 `T` 零值做 `switch any(z).(type)` 完成类型分派。编译期分派需要
  为每个类型引一个函数，反而把分支推到每个调用点。因为只在解码边界调用，
  当前形态可以接受。
- `OverflowUint` 从不检查 `uint64`（入参已经是 `uint64`），
  `OverflowInt[int64]` 同理，都是空操作。这是有意的：调用方直接把 `strconv`
  产出的 `int64` / `uint64` 传进来，"不需要截断"的分支必须便宜。
- `OverflowFloat[float64]` 恒返回 `false`；`OverflowFloat[float32]` 与
  `±math.MaxFloat32` 比较。次正规数和 NaN 不当作溢出。
