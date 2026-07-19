# fileutil
[English](README.md) | [中文](README_CN.md)

`fileutil` provides two tiny filesystem helpers that plug small gaps in the
standard library's `os` package. Part of Go-Spring's zero-dependency `stdlib`
layer.

## Features

- `PathExists(path)` — `(true, nil)` if the path exists, `(false, nil)` when
  it does not, `(false, err)` on any other error (e.g. permission denied).
- `ReadDirNames(dirname)` — returns the names of every entry in `dirname`,
  order defined by the filesystem.

## Usage

```go
import "go-spring.org/stdlib/fileutil"

ok, err := fileutil.PathExists("/etc/app.conf")
if err != nil {
    return err
}
if !ok {
    // absent
}

names, err := fileutil.ReadDirNames("/var/log/app")
```
