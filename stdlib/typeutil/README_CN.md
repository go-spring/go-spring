# typeutil
[English](README.md) | [中文](README_CN.md)

`typeutil` 提供 Go-Spring 容器用来判定类型的反射工具集（bean 还是属性、
是不是构造器签名、基本类型还是结构体等）。属于零依赖的 `stdlib` 层。

## API

类型约束：

- `IntType`、`UintType`、`FloatType` —— 对应 Go 数值族的泛型约束。

`reflect.Type` 上的判定函数：

- `IsFuncType(t)` —— 是否函数类型。
- `IsErrorType(t)` —— 是不是 `error` 或实现了 `error`。
- `ReturnNothing(t)` —— 函数无返回值。
- `ReturnOnlyError(t)` —— 函数只返回一个 error。
- `IsConstructor(t)` —— 单返回值非 error，或双返回值第二个是 error。
- `IsPrimitiveValueType(t)` —— int / uint / float / string / bool。
- `IsPropBindingTarget(t)` —— 属性绑定的合法目标（基本类型、结构体、
  或元素为上述类型的集合）。
- `IsBeanType(t)` —— chan、func、interface、或 `*struct`。
- `IsBeanInjectionTarget(t)` —— bean 类型，或元素为 bean 的集合。

## 用法

```go
import (
    "reflect"
    "go-spring.org/stdlib/typeutil"
)

if typeutil.IsConstructor(reflect.TypeOf(fn)) {
    // 可以按构造器注册
}
```

主要被 Go-Spring 容器在扫描 autowire / provider 目标时使用。
