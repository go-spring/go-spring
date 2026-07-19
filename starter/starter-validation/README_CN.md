# starter-validation

[English](README.md) | [中文](README_CN.md)

`starter-validation` 把 [go-playground/validator][gpv] 注册为
[`stdlib/validation`](../../stdlib/validation) 抽象的 `default` driver。空导入
后,任何 `validation.Validate(ctx, "default", v)` 调用——无论来自 `conf.Bind`
校验后置,还是 Web 请求处理——都会用带 `validate:"..."` 标签的结构体规则完成
校验。

它属于 *global / infrastructure*(全局 / 基础设施)形态(见
[starter/DESIGN.md](../DESIGN.md) §2.4):不注册 bean,也不开监听端口。第三方
校验器只在这里引入——绝不进 `stdlib`——从而让基础层保持零依赖承诺。

[gpv]: https://github.com/go-playground/validator

## 安装

```bash
go get go-spring.org/starter-validation
```

## 快速开始

### 1. 导入 starter

```go
import _ "go-spring.org/starter-validation"
```

### 2. 打上校验标签

`go-playground/validator` 支持的所有标签(`required`、`email`、`min=...`、
`oneof=a b c` 等)都能直接使用。

```go
type SignupRequest struct {
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   validate:"min=18"`
}
```

### 3. 校验

```go
import "go-spring.org/stdlib/validation"

if err := validation.Validate(ctx, "default", &req); err != nil {
    // err 是 validation.ValidationErrors——每条失败规则一个 FieldError
}
```

`ValidationErrors` 是中立结构:每条包含 `Field`(结构体 namespace)、`Rule`
(validator 标签,如 `email`)、`Param`(标签参数,如 `min=18` 中的 `18`)以及
`Value`(触发错误的值)。标签本身即 i18n key:`validation.<tag>`。

## i18n

搭配 [`stdlib/i18n`](../../stdlib/i18n) 可做本地化输出,不用硬编码消息:

```properties
# messages_en.yaml
validation.required: "{{.Field}} is required"
validation.email:    "{{.Field}} must be a valid email"
validation.min:      "{{.Field}} must be at least {{.Param}}"
```

```go
lines := err.(validation.ValidationErrors).Localize(func(fe validation.FieldError) string {
    s, _ := src.Message(ctx, fe.MessageKey(), fe.Field, fe.Param)
    return s
})
```

见 [`example/`](example) 的可运行示例——端到端演示 `conf.Bind` 风格的配置
路径与 HTTP 处理路径,含中英文消息包。

## 非标签错误

`InvalidValidationError`(如把 nil 指针传给 `Validate`)属于编程错误而非字段
失败,会原样返回,不会被包成 `ValidationErrors`。
