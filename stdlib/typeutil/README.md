# typeutil
[English](README.md) | [中文](README_CN.md)

`typeutil` provides reflection helpers used by the Go-Spring container to
classify types (bean vs. property, constructor signatures, primitive vs.
struct). Part of the zero-dependency `stdlib` layer.

## API

Type constraints:

- `IntType`, `UintType`, `FloatType` — generic constraints over the
  respective Go primitive number families.

Reflection predicates on `reflect.Type`:

- `IsFuncType(t)` — is `t` a function type?
- `IsErrorType(t)` — is `t` exactly `error` or does it implement `error`?
- `ReturnNothing(t)` — function returns no values.
- `ReturnOnlyError(t)` — function returns exactly one value, which is an
  error.
- `IsConstructor(t)` — function returns either one non-error value or two
  values where the second is an error.
- `IsPrimitiveValueType(t)` — is it int / uint / float / string / bool?
- `IsPropBindingTarget(t)` — valid target for property binding (primitive,
  struct, or collection of those).
- `IsBeanType(t)` — chan, func, interface, or `*struct`.
- `IsBeanInjectionTarget(t)` — bean type or collection of beans.

## Usage

```go
import (
    "reflect"
    "go-spring.org/stdlib/typeutil"
)

if typeutil.IsConstructor(reflect.TypeOf(fn)) {
    // ok, register as a bean constructor
}
```

Consumed primarily by the Go-Spring container when scanning
autowire / provider targets.
