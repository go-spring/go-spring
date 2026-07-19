# listutil 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖的 `stdlib` 层。`listutil` 是 `container/list` 的极薄泛型封装，
外加几个切片 / Writer 便捷函数。

## 1. 职责与边界

- 给 `container/list` 补回编译期类型信息，同时不重写数据结构本身。每个
  方法都是内嵌 stdlib 类型上的一行转发。
- 提供框架代码里高频出现的小工具（`SliceOf`、`ListOf`、`AllOfList`、
  `WriteStrings`）。
- 不是链表重写，也不是函数式集合库。

## 2. 设计说明

- `Element[T]` 通过指针嵌入 `*list.Element`，`Valid()` 直接判空即可。
  `Element[T]` 的零值本身就是遍历结束的"nil"标记。
- `AllOfList` 走 `e.Value.(T)`，混类型时直接 panic。这是有意为之 —— 检查
  版本得引入 `ok, err`，与多数调用点期望不符。
- 该泛型封装**不会**检查传入的 `Element[T]` 是否来自别的链表 ——
  `container/list` 本身会 panic，封装层没有额外拦截。
