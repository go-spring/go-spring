# listutil

[English](README.md) | [中文](README_CN.md)

`listutil` 给 Go 的 `container/list` 加上泛型类型安全外壳，并补充一些切片、Writer 相关的便捷函数。

## 使用方式

导入路径：`go-spring.org/stdlib/listutil`。

```go
import "go-spring.org/stdlib/listutil"

l := listutil.New[int]()
l.PushBack(1)
l.PushBack(2)

for e := l.Front(); e.Valid(); e = e.Next() {
    _ = e.Value() // int
}
```

API 概览：

- `List[T]` / `Element[T]`——对 `container/list.List` / `list.Element` 的泛型薄封装，保持双向链表 API，同时把 `any` 换成具体类型。
- 便捷函数：
  - `SliceOf[T](items ...T) []T`——用变参构造切片的语法糖。
  - `ListOf[T](items ...T) *list.List`——用变参构造 `*list.List`。
  - `AllOfList[T](l *list.List) []T`——把 list 元素收集为 `[]T`（元素类型不匹配会 panic）。
  - `WriteStrings(w io.Writer, values ...string) error`——依次写入字符串，首个错误即停止。

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。
