# iterutil 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖的 `stdlib` 层。`iterutil` 是一组通过回调驱动的循环，让 `defer`
拥有"每次迭代"的语义。

## 1. 职责与边界

- 提供 `Times`、`Ranges`、`StepRanges`，每种把循环体交给一个回调函数，
  于是回调内的 `defer` 在回调返回时立即触发，而不是等到外层函数返回。
- 不是完整的迭代 DSL。绝大多数循环用原生 `for` 就够了；只有真正需要按次
  清理时才用这里的工具。

## 2. 设计说明

- 方向由参数推断。`Ranges(2, 5, fn)` 正向，`Ranges(5, 2, fn)` 反向；这样
  省掉一个布尔参数，代价是 `start == end` 时不执行。
- `StepRanges` 要求 `step` 的符号与区间方向匹配，不匹配则一次都不调用，
  而不是死循环。
- 不提供返回 `error` 的变体。需要提前退出请回到原生 `for` 循环。
