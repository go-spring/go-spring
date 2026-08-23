# textstyle

[English](README.md) | [中文](README_CN.md)

`textstyle` 用 ANSI 转义序列为字符串加颜色和样式，用于终端输出。它永远输出转义码、不检测目标是不是终端，适合本身就知道该不该加样式的 CLI 工具与日志辅助逻辑。

## 使用方式

```go
import "go-spring.org/stdlib/textstyle"

fmt.Println(textstyle.Red.Sprint("error: connection refused"))
fmt.Println(textstyle.NewText(textstyle.Bold, textstyle.Green).
    Sprintf("ok %d/%d", n, total))
```

### API 列表

- 样式属性：`Bold`、`Italic`、`Underline`、`ReverseVideo`、`CrossedOut`。
- 前景色：`Black`、`Red`、`Green`、`Yellow`、`Blue`、`Magenta`、`Cyan`、`White`。
- 背景色：`BgBlack`、`BgRed`、`BgGreen`、`BgYellow`、`BgBlue`、`BgMagenta`、`BgCyan`、`BgWhite`。
- `Attribute.Sprint(a ...any)` / `Attribute.Sprintf(format, a ...any)` 用于单一属性。
- 用 `NewText(attributes ...Attribute)` 构造的 `Text` 用于组合多个属性。

包裹后的输出形如 `\x1b[<codes>m<text>\x1b[0m`。写入非终端时，需要调用方自行剥除转义序列 —— 本包不做终端检测。

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。
