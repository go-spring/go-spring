# netutil
[English](README.md) | [中文](README_CN.md)

`netutil` provides tiny network helpers for use inside the Go-Spring
framework. Part of the zero-dependency `stdlib` layer.

## API

- `LocalIPv4() string` — the first non-loopback IPv4 address of the local
  machine, or `"0.0.0.0"` when none is available. Cached after the first
  call.

## Usage

```go
import "go-spring.org/stdlib/netutil"

ip := netutil.LocalIPv4()
```

## Caveats

- IPv6 is ignored.
- The result is cached at the first call via `sync.Once`; later interface
  changes are not observed.
- Errors from `net.InterfaceAddrs()` are swallowed — the fallback address
  is `"0.0.0.0"`.
