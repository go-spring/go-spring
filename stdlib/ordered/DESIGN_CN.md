# ordered 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖的 `stdlib` 层。`ordered` 目前只有一个函数，这个包的存在是为
未来"稳定遍历顺序"类工具提供一个命名归口。

## 1. 职责与边界

- 提供一步到位的"以稳定顺序遍历 map"，避免每个调用点重复写 `sort.Strings` +
  临时切片。
- 不是有序 map 容器。当前场景用原生 map + 这个 helper 就够了，因此不做
  插入顺序容器。

## 2. 设计说明

- 使用 `cmp.Ordered` 约束（Go 1.21+），一次覆盖数值和字符串 key，避免为不同
  key 类型重复写函数。
- 返回切片是独立拷贝，调用方可以随意修改。
- 内部走 `slices.Sort` 而非 `sort.Strings`，保持实现泛型。
