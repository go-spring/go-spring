# i18n
[English](README.md) | [中文](README_CN.md)

`i18n` 是零依赖的本地化消息抽象。`MessageSource` 将 key + 参数解析为调用者
语言下的字符串;它同时支撑业务文案和 `validation.ValidationErrors` 的渲染,
而无需在基础层引入任何第三方 i18n 库。

## 特性

- 零第三方依赖。
- Locale 通过 `context.Context` 传递(`WithLocale` / `LocaleFrom`),与 trace
  上下文的传递方式一致。
- 内置 `MapSource` 在内存中按 locale → key 存储模板。
- 回退查找顺序:请求 locale → 默认 locale → 原 key + `ErrMessageNotFound`
  (调用方选择 fail-loud 或优雅降级)。
- 位置占位符 `{0}`、`{1}`、...;未匹配占位符原样保留,让模板不一致显现而非
  静默丢失。
- 接收已解析好的 map(`AddMap` 扁平、`AddParsed` 嵌套);同时处理
  `map[string]any`(json)与 `map[any]any`(yaml.v2)嵌套,按点号 flatten。
- `Localizer(src, ctx)` 生成 `func(key, args...) string` 签名,
  `validation.ValidationErrors.Localize` 需要的正是这个;缺 key 返回 `""`,
  由调用方使用默认消息兜底。

## 快速开始

Import 路径: `go-spring.org/spring/i18n`。

```go
package main

import (
    "context"
    "fmt"

    "go-spring.org/spring/i18n"
)

func main() {
    src := i18n.NewMapSource("en").
        Add("en", "hello", "Hello, {0}!").
        Add("zh", "hello", "你好, {0}!")

    ctx := i18n.WithLocale(context.Background(), "zh")
    msg, _ := src.Message(ctx, "hello", "Go-Spring")
    fmt.Println(msg) // "你好, Go-Spring!"
}
```

对由外部 parser 加载的嵌套 bundle(`spring/conf` reader、远程配置中心
provider ...),把解析后的 map 直接喂给 `AddParsed`:

```go
src.AddParsed("en", map[string]any{
    "validation": map[string]any{
        "email": "{0} must be a valid email",
    },
})
```
