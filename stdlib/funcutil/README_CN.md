# funcutil
[English](README.md) | [中文](README_CN.md)

`funcutil` 提供 Go 函数值的运行时元信息（文件、行号、名字）。容器 / 切面
框架用它拼装人类可读的诊断信息。属于零依赖的 `stdlib` 层。

## API

- `FuncName(fn any) string` —— 去掉模块路径前缀后的包限定函数名。
  运行时对方法值打印为 `T.m-fm`，此处会去掉尾部 `-fm`。
- `FileLine(fn any) (file string, line int, fnName string)` —— 源码位置
  加上清理后的函数名。

## 用法

```go
import "go-spring.org/stdlib/funcutil"

func Handle() {}

name := funcutil.FuncName(Handle)
file, line, _ := funcutil.FileLine(Handle)
```

`fn` 必须是函数或方法值，传其它类型会在 `reflect` 内部 panic。
