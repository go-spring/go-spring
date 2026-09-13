# i18n
[English](README.md) | [中文](README_CN.md)

`i18n` 做本地化消息解析:`MessageSource` 把 key + 参数变成调用方语言的字
符串。它同时服务面向用户的业务文案和 `validation.ValidationErrors` 的渲
染,零第三方依赖。

## 使用方式

### 1. 按请求 locale 解析消息

```go
src := i18n.NewMapSource(i18n.WithDefaultLocale("en")).
    AddMessage("en", "hello", "Hello, {0}!").
    AddMessage("zh", "hello", "你好, {0}!")

ctx := i18n.WithLocale(context.Background(), "zh")
msg, _ := src.Message(ctx, "hello", "Go-Spring")
fmt.Println(msg) // 你好, Go-Spring!
```

查找顺序固定:请求 locale → 默认 locale → 返回 key 本身并附带包装
`ErrMessageNotFound` 的错误。要优雅降级就忽略错误,要 fail loud 就
`errors.Is`。

### 2. 喂入别处解析好的 bundle

本包不读文件——解析器属于接线层(spring/conf reader、远程配置中心)。
把解析出的 map 交进来;key 就是 map 里写的名字(点连接只是惯例,不是要
求)。`MapSource` 是内建的内存实现;要换成数据库/配置中心,实现唯一那个方
法 `Message(ctx, key, args...) (string, error)` 即可:

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

## 关键设计

- locale 随 context 传递(`WithLocale` / `LocaleFrom`):中间件从
  `Accept-Language` 设置一次,下游每次 `Message` 调用自动取到——和 trace
  context 的流动方式一样。
- 位置插值 `{0}`、`{1}`、...;没有对应参数的占位符原样保留——模板漂移
  看得见,而不是悄悄消失。
- 不是 ICU MessageFormat:没有复数/性别的 DSL。位置插值足够覆盖校验消
  息与常见业务文案。
- 零依赖;locale 的 context key 未导出,不会与其他包的 key 冲突。

## 许可证

`i18n` 基于 Apache License 2.0 发布，详见 [LICENSE](../../LICENSE)。
