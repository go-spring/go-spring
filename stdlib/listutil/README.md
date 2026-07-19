# listutil
[English](README.md) | [中文](README_CN.md)

`listutil` gives Go's `container/list` a generic, type-safe skin and adds
a few convenience helpers for slices and writers. Part of the zero-dependency
`stdlib` layer.

## Features

- `List[T]` / `Element[T]` — thin generic wrappers over `container/list.List`
  / `list.Element`, keeping the doubly-linked-list API but returning typed
  values instead of `any`.
- Helper functions:
  - `SliceOf[T](items ...T) []T` — sugar for building a slice from varargs.
  - `ListOf[T](items ...T) *list.List` — build a `*list.List` from varargs.
  - `AllOfList[T](l *list.List) []T` — collect all elements as `[]T`
    (panics if the list holds a different type).
  - `WriteStrings(w io.Writer, values ...string) error` — write strings in
    order, stopping at the first error.

## Usage

```go
import "go-spring.org/stdlib/listutil"

l := listutil.New[int]()
l.PushBack(1)
l.PushBack(2)

for e := l.Front(); e.Valid(); e = e.Next() {
    _ = e.Value() // int
}
```
