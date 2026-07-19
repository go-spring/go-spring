# fileutil
[English](README.md) | [中文](README_CN.md)

`fileutil` 提供两个针对 `os` 包空白点的文件系统小工具，属于 Go-Spring 零依赖
`stdlib` 层。

## 功能

- `PathExists(path)` —— 路径存在返回 `(true, nil)`，不存在返回
  `(false, nil)`，出现其它错误（如权限不足）返回 `(false, err)`。
- `ReadDirNames(dirname)` —— 返回目录下所有条目名，顺序由文件系统决定。

## 用法

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
