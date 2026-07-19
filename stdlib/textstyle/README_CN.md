# textstyle
[English](README.md) | [中文](README_CN.md)

`textstyle` 用 ANSI 转义序列为字符串加颜色和样式，用于终端输出。属于
Go-Spring 零依赖的 `stdlib` 层。

## 功能

- 样式属性：`Bold`、`Italic`、`Underline`、`ReverseVideo`、`CrossedOut`。
- 前景色：`Black`、`Red`、`Green`、`Yellow`、`Blue`、`Magenta`、`Cyan`、
  `White`。
- 背景色：`BgBlack`、`BgRed`、`BgGreen`、`BgYellow`、`BgBlue`、`BgMagenta`、
  `BgCyan`、`BgWhite`。
- `Attribute.Sprint(a ...any)` / `Attribute.Sprintf(format, a ...any)` 用于
  单一属性。
- 用 `NewText(attributes ...Attribute)` 构造的 `Text` 用于组合多个属性。

## 用法

```go
import "go-spring.org/stdlib/textstyle"

fmt.Println(textstyle.Red.Sprint("error: connection refused"))
fmt.Println(textstyle.NewText(textstyle.Bold, textstyle.Green).
    Sprintf("ok %d/%d", n, total))
```

包裹后的输出形如 `\x1b[<codes>m<text>\x1b[0m`。写入非终端时，需要调用方自行
剥除转义序列 —— 本包不做终端检测。
