# netutil
[English](README.md) | [中文](README_CN.md)

`netutil` 提供供 Go-Spring 框架内部使用的网络小工具。属于零依赖的 `stdlib`
层。

## API

- `LocalIPv4() string` —— 本机第一个非回环 IPv4 地址，找不到时返回
  `"0.0.0.0"`。首次调用后缓存。

## 用法

```go
import "go-spring.org/stdlib/netutil"

ip := netutil.LocalIPv4()
```

## 注意事项

- 忽略 IPv6。
- 结果通过 `sync.Once` 在首次调用时缓存，之后接口变化不会被察觉。
- `net.InterfaceAddrs()` 的错误被吞掉，回退值为 `"0.0.0.0"`。
