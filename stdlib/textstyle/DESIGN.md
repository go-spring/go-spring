# textstyle Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. `textstyle` is a compact ANSI
wrapper used by CLI tooling (`gs`) and log helpers.

## 1. Responsibilities & Boundaries

- Emit ANSI-wrapped strings for the small, common set of styles and colors
  the framework needs. Predefined codes are hard-coded constants so the
  file is a single lookup + writer.
- Not a full terminal library. No cursor movement, no clearing, no 256- /
  true-color support. If those are needed, reach for `fatih/color` or
  `charmbracelet/lipgloss` — outside of stdlib.

## 2. Design Notes

- Two entry shapes: `Attribute.Sprint(f)` for a single attribute and
  `Text.Sprint(f)` for combinations. Both go through the same `wrap`
  helper, which writes `\x1b[a;b;c]m...\x1b[0m` and never omits the reset.
- No auto-detection of TTY. Callers know whether they are writing to a
  terminal or a file / pipe; smuggling that detection into a leaf utility
  package would push a dependency (`golang.org/x/term`) into stdlib.
- No mutable global state. Every call is stateless, so it is safe to call
  from any goroutine.
