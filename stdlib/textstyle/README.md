# textstyle

[English](README.md) | [中文](README_CN.md)

`textstyle` wraps strings with ANSI escape codes for colored / styled terminal output. It always emits the codes and never detects whether the destination is a terminal, which suits CLI tooling and log helpers that already know whether styling is appropriate.

## Usage

```go
import "go-spring.org/stdlib/textstyle"

fmt.Println(textstyle.Red.Sprint("error: connection refused"))
fmt.Println(textstyle.NewText(textstyle.Bold, textstyle.Green).
    Sprintf("ok %d/%d", n, total))
```

### API

- Style attributes: `Bold`, `Italic`, `Underline`, `ReverseVideo`, `CrossedOut`.
- Foreground colors: `Black`, `Red`, `Green`, `Yellow`, `Blue`, `Magenta`, `Cyan`, `White`.
- Background colors: `BgBlack`, `BgRed`, `BgGreen`, `BgYellow`, `BgBlue`, `BgMagenta`, `BgCyan`, `BgWhite`.
- `Attribute.Sprint(a ...any)` / `Attribute.Sprintf(format, a ...any)` for single attributes.
- `Text` type built from `NewText(attributes ...Attribute)` for combined attributes.

Wrapped output is `\x1b[<codes>m<text>\x1b[0m`. When targeting a non-terminal writer, callers should strip the sequences themselves — this package does not detect terminals.

## License

Apache License 2.0. See [LICENSE](../../LICENSE).
