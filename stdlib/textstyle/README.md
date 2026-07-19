# textstyle
[English](README.md) | [中文](README_CN.md)

`textstyle` wraps strings with ANSI escape codes for colored / styled
terminal output. Part of Go-Spring's zero-dependency `stdlib` layer.

## Features

- Style attributes: `Bold`, `Italic`, `Underline`, `ReverseVideo`,
  `CrossedOut`.
- Foreground colors: `Black`, `Red`, `Green`, `Yellow`, `Blue`, `Magenta`,
  `Cyan`, `White`.
- Background colors: `BgBlack`, `BgRed`, `BgGreen`, `BgYellow`, `BgBlue`,
  `BgMagenta`, `BgCyan`, `BgWhite`.
- `Attribute.Sprint(a ...any)` / `Attribute.Sprintf(format, a ...any)` for
  single attributes.
- `Text` type built from `NewText(attributes ...Attribute)` for combined
  attributes.

## Usage

```go
import "go-spring.org/stdlib/textstyle"

fmt.Println(textstyle.Red.Sprint("error: connection refused"))
fmt.Println(textstyle.NewText(textstyle.Bold, textstyle.Green).
    Sprintf("ok %d/%d", n, total))
```

Wrapped output is `\x1b[<codes>m<text>\x1b[0m`. When targeting a
non-terminal writer, callers should strip the sequences themselves — this
package does not detect terminals.
