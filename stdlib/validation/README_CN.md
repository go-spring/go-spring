# validation
[English](README.md) | [中文](README_CN.md)

`validation` 是与框架无关、零依赖的结构体校验抽象,与 `stdlib/resilience`、
`stdlib/discovery` 同款范式:抽象与实现拆开。它回答"这个 struct 是否合法?"
——同时服务配置绑定与入站 Web 请求两条路径。

## 特性

- 抽象层零第三方依赖。
- 中立单元 `FieldError{Field, Rule, Param, Value}` 与 `ValidationErrors` 列
  表是每个 driver 必须产出的。
- `Validator` 接口 + `Driver` 工厂 + 注册表(`RegisterDriver` / `GetDriver`
  / `MustGetDriver`);`starter-validation` 在 blank import 时把
  `go-playground/validator` 注册为 `"default"` driver。
- `ValidationErrors.Localize(msg func(key, args...) string)` 通过任意查询函
  数渲染逐字段消息——通常用 `stdlib/i18n` 的 `i18n.Localizer(src, ctx)` 绑
  定。本包不直接 import i18n。
- Web 缝隙:泛型 `Handle[T](v, decode, render, next)` 返回 `http.Handler`——
  先解码、后校验,校验失败以结构化 JSON 400 短路,不让脏数据进入业务代码。默
  认解码器 `JSONDecoder`。
- 便捷函数 `Validate(ctx, name, v)`——配置绑定路径一次性使用(解析 driver、
  建 validator、校验);热路径请复用 `Validator`。

## 快速开始

Import 路径: `go-spring.org/stdlib/validation`。

```go
package main

import (
    "context"
    "log"
    "net/http"

    "go-spring.org/stdlib/validation"
    _ "go-spring.org/starter/starter-validation" // 注册 "default" driver
)

type SignUp struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}

func main() {
    d, _ := validation.MustGetDriver("default")
    v, _ := d.NewValidator()

    h := validation.Handle[SignUp](v, nil, nil, func(w http.ResponseWriter, r *http.Request, in *SignUp) {
        _, _ = w.Write([]byte("ok"))
    })
    log.Fatal(http.ListenAndServe(":8080", h))

    _ = context.Background // 提示: 搭配 i18n.Localizer 做本地化错误
}
```

要按调用者语言渲染错误,传入使用 `i18n.Localizer` 的 `render`:

```go
render := func(fe validation.FieldError) string {
    // fe.MessageKey() == "validation." + fe.Rule
    return i18n.Localizer(src, ctx)(fe.MessageKey(), fe.Field, fe.Param)
}
```
