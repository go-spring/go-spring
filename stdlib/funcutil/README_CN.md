# funcutil

[English](README.md) | [中文](README_CN.md)

`funcutil` 提供 Go 函数值的运行时元信息（文件、行号、名字），供容器 / 切面
框架拼装人类可读的诊断信息。

## 使用方式

```go
import "go-spring.org/stdlib/funcutil"

func Handle() {}

name := funcutil.FuncName(Handle)
file, line, _ := funcutil.FileLine(Handle)
```

`fn` 必须是函数或方法值，传其它类型会在 `reflect` 内部 panic。

### API 列表

- `FuncName(fn any) string` —— 去掉模块路径前缀后的包限定函数名。
  运行时对方法值打印为 `T.m-fm`，此处会精确去掉尾部 `-fm` 后缀；恰好以
  `-`、`f`、`m` 结尾的函数名不受影响。
- `FileLine(fn any) (file string, line int, fnName string)` —— 源码位置
  加上清理后的函数名。

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。
