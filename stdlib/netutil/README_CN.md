# netutil

[English](README.md) | [中文](README_CN.md)

`netutil` 提供 Go-Spring 框架内部使用的网络小工具——当前只有注册与启动日志
用到的本机 IPv4 查询。不是网络框架——接口
枚举、CIDR 匹配、地址解析已由 `net` 覆盖。

## 使用方式

```go
import "go-spring.org/stdlib/netutil"

ip := netutil.LocalIPv4()
```

### API 列表

- `LocalIPv4() string` —— 本机第一个非回环 IPv4 地址，找不到时返回
  `"0.0.0.0"`。首次调用后缓存。

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。
