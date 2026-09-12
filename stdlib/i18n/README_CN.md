# i18n
[English](README.md) | [中文](README_CN.md)

`i18n` 做本地化消息解析:`MessageSource` 把 key + 参数变成调用方语言的字
符串。它同时服务面向用户的业务文案和 `validation.ValidationErrors` 的渲
染,零第三方依赖。

## API

| API | 作用 |
| --- | --- |
| `MessageSource` | 唯一的接口:`Message(ctx, key, args...) (string, error)`。想接数据库/配置中心,自己实现它。 |
| `WithLocale(ctx, locale)` / `LocaleFrom(ctx)` | locale 随 context 传递——中间件从 `Accept-Language` 设一次,下游所有调用自动取到。 |
| `NewMapSource(WithDefaultLocale)` | 内建内存后端:locale → key → 模板。`Add` 注册单条模板,`AddBundle` 注册整个 locale 的 bundle(可链式)。 |
| `Localizer(src, ctx)` | 把 `MessageSource` 柯里化成 `func(key, args...) string`——正是 `validation.ValidationErrors.Localize` 要的形状。 |
| `ErrMessageNotFound` | "key 不存在"错误包装的哨兵。缺失时返回的字符串就是 key 本身。 |

## 用法

### 1. 按请求 locale 解析消息

```go
src := i18n.NewMapSource(WithDefaultLocale("en")).
    Add("en", "hello", "Hello, {0}!").
    Add("zh", "hello", "你好, {0}!")

ctx := i18n.WithLocale(context.Background(), "zh")
msg, _ := src.Message(ctx, "hello", "Go-Spring")
fmt.Println(msg) // 你好, Go-Spring!
```

查找顺序固定:请求 locale → 默认 locale → 返回 key 本身并附带包装
`ErrMessageNotFound` 的错误。要优雅降级就忽略错误,要 fail loud 就
`errors.Is`。

### 2. 喂入别处解析好的 bundle

本包不读文件——解析器属于接线层(spring/conf reader、远程配置中心)。
把解析出的 map 交进来;key 就是 map 里写的名字(点连接只是惯例,不是要求):

```go
src.AddBundle("en", map[string]string{
    "validation.email": "{0} must be a valid email",
})
```

### 3. 配合 validation 错误

`Localizer` 产出的正是 `validation.ValidationErrors.Localize` 消费的查
找签名;key 缺失时返回 `""`,`Localize` 会回落 `FieldError.Default()`:

```go
msgs := errs.Localize(i18n.Localizer(src, ctx))
```

## 模型保证的规则

- 位置插值 `{0}`、`{1}`、...;没有对应参数的占位符原样保留——模板漂移
  看得见,而不是悄悄消失。
- 不是 ICU MessageFormat:没有复数/性别的 DSL。位置插值足够覆盖校验消
  息与常见业务文案。
- 零依赖;locale 的 context key 未导出,不会与其他包的 key 冲突。

## 许可证

`i18n` 基于 Apache License 2.0 发布，详见 [LICENSE](../../LICENSE)。
