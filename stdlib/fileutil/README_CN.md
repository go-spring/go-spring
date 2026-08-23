# fileutil

[English](README.md) | [中文](README_CN.md)

`fileutil` 提供两个针对 `os` 包空白点的文件系统小工具。它不是文件系统抽象层：遍历、监听、原子写、路径拼接都不在这里——它们要么在 `os` / `filepath` 中，要么属于更上层的包。

## 使用方式

```go
import "go-spring.org/stdlib/fileutil"

ok, err := fileutil.PathExists("/etc/app.conf")
if err != nil {
    return err
}
if !ok {
    // 不存在
}

names, err := fileutil.ReadDirNames("/var/log/app")
```

### API 列表

- `PathExists(path) (bool, error)` —— 路径存在返回 `(true, nil)`，不存在
  返回 `(false, nil)`，出现其它错误（如权限不足）返回 `(false, err)`。
- `ReadDirNames(dirname) ([]string, error)` —— 返回目录下所有条目名，
  顺序由文件系统决定。

## 许可证

`fileutil` 基于 Apache License 2.0 发布，详见 [LICENSE](../../LICENSE)。
