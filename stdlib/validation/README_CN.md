# validation

[English](README.md) | [中文](README_CN.md)

`validation` 是结构体校验的中立错误模型:扁平的 `ValidationErrors` 列表,
元素是 `FieldError{Field, Rule, Param, Kind, Value}`。校验器随你选(通常直接用
go-playground/validator);失败时把它的错误映射到这个形状,渲染和本地化就统一了。

## 什么时候用

- 你用 go-playground/validator(或任何校验器)做校验,需要把失败渲染成
  面向用户的消息,可能还要多语言。
- 应用里多处需要以同一个稳定形状(字段路径 + 规则 + 参数)报告校验失败。

如果只是 `err != nil` 然后打日志,不需要本包。

## 使用方式

### 1. 用你的校验器校验,失败时映射

```go
import (
    "github.com/go-playground/validator/v10"
    "go-spring.org/stdlib/validation"
)

v := validator.New(validator.WithRequiredStructEnabled())

err := v.Struct(&SignUp{Email: "not-an-email"})
verrs, ok := err.(validator.ValidationErrors)
if !ok {
    return err // 真错误,不是字段失败
}
out := make(validation.ValidationErrors, len(verrs))
for i, fe := range verrs {
    out[i] = validation.FieldError{
        Field: fe.Namespace(), // struct 路径 → 稳定标识
        Rule:  fe.Tag(),       // "email" → 键 "validation.email"
        Param: fe.Param(),     // min=3 时的 "3"
        Kind:  fe.Kind().String(), // "string" / "int" / ...
    }
}
```

### 2. 配合 stdlib/i18n 按调用方语言渲染

按规则准备消息:

```go
src := i18n.NewMapSource(i18n.WithDefaultLocale("en")).
    AddMessage("zh-CN", "validation.email", "{0} 不是合法邮箱").
    AddMessage("zh-CN", "validation.min", "{0} 至少为 {1}")
```

按请求本地化(locale 随 context 传递):

```go
msgs := out.Localize(i18n.Localizer(src, ctx))
// []string{"SignUp.Email 不是合法邮箱", ...}
```

缺翻译回落 `FieldError.Default()`,输出永不为空。

### 3. 定制:按字段改话术、按类型消歧

`Localize` 的可选回调拿到整个 `FieldError`。返回非空串即覆盖,返回 `""`
落回通用模板。用它给某个字段单独写话术,或消歧"同名规则在不同类型上含义
不同"——`min` 作用在 string 上是长度、作用在 number 上是数值:

```go
msgs := out.Localize(i18n.Localizer(src, ctx), validation.WithCustom(func(fe validation.FieldError) string {
    if fe.Field == "SignUp.Password" {
        return "密码至少 8 位"
    }
    if fe.Rule == "min" && fe.Kind == "string" {
        return fe.Field + " 长度至少为 " + fe.Param
    }
    return "" // 通用模板
}))
```

### 4. 或直接当普通 error 用

`ValidationErrors` 实现 `error`,拼接默认消息:

```go
return out // "validation: field \"SignUp.Email\" failed on the \"email\" rule"
```

## 关键设计

- 消息键遵循 `FieldError.MessageKey()`:`"validation." + Rule`,规则 `email`
  查的是 `validation.email`。
- `Field` 是 struct 字段路径,不是 JSON tag;渲染层可自行改写。
- `Localize` 每条失败返回一个字符串、顺序与列表一致,且永不为空。
- 本包零第三方依赖——从校验器错误类型的映射永远在调用侧,几行而已。

## 许可证

`validation` 基于 Apache License 2.0 发布，详见 [LICENSE](../../LICENSE)。
