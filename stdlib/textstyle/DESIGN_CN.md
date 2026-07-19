# textstyle 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖的 `stdlib` 层。`textstyle` 是一个小型 ANSI 封装，供 CLI 工具
（`gs`）和日志辅助逻辑使用。

## 1. 职责与边界

- 输出 ANSI 包装后的字符串，覆盖框架需要的小组样式和颜色集。所有 ANSI 码
  直接作为常量写死，整个文件本质上是一次查表 + 一次写入。
- 不是完整终端库。不做光标控制、清屏、256 色 / 真彩色支持。有这类需求
  请引入 `fatih/color`、`charmbracelet/lipgloss` 等 stdlib 之外的库。

## 2. 设计说明

- 两种入口：`Attribute.Sprint(f)` 用于单属性，`Text.Sprint(f)` 用于组合。
  两者共用 `wrap`，统一输出 `\x1b[a;b;c]m...\x1b[0m`，并且永远带上 reset。
- 不做 TTY 自动检测。调用方本身清楚在写终端还是文件 / 管道；把这层检测
  塞进底层工具包会把 `golang.org/x/term` 依赖引入 stdlib。
- 没有可变全局状态，任意 goroutine 都能安全调用。
