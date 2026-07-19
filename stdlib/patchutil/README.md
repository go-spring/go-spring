# patchutil
[English](README.md) | [中文](README_CN.md)

`patchutil` exposes a single reflection helper that clears the internal
read-only flag on a `reflect.Value`, allowing assignment to unexported
struct fields. Intended for **internal tooling and tests only**. Part of
Go-Spring's zero-dependency `stdlib` layer.

## API

- `PatchValue(v reflect.Value) reflect.Value` — returns the same `Value`
  with its `flagRO` bits cleared, so a following `Set` call succeeds even
  when the value originally addressed an unexported field.

## Usage

```go
import (
    "reflect"
    "go-spring.org/stdlib/patchutil"
)

f := patchutil.PatchValue(reflect.ValueOf(&obj).Elem().FieldByName("secret"))
f.SetString("new value")
```

## Warning

- Uses `unsafe` to poke at `reflect.Value`'s internal `flag` field.
- Highly version-dependent; may break on future Go releases.
- Do **not** use in production business code — reserve for framework-internal
  tooling and tests where the alternative is worse.
