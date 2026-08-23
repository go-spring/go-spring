# patchutil

[English](README.md) | [中文](README_CN.md)

`patchutil` exposes a single reflection helper that clears the internal
read-only flag on a `reflect.Value`, allowing assignment to unexported
struct fields. It exists for framework-internal seams that cannot change
the target type, and is intended for **internal tooling and tests only**.

## Usage

```go
import (
    "reflect"
    "go-spring.org/stdlib/patchutil"
)

f := patchutil.PatchValue(reflect.ValueOf(&obj).Elem().FieldByName("secret"))
f.SetString("new value")
```

### API

- `PatchValue(v reflect.Value) reflect.Value` — returns the same `Value`
  with its `flagRO` bits cleared, so a following `Set` call succeeds even
  when the value originally addressed an unexported field.

## License

Apache License 2.0. See [LICENSE](../../LICENSE).
