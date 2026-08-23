# netutil

[English](README.md) | [中文](README_CN.md)

`netutil` provides tiny network helpers for use inside the Go-Spring
framework — currently just the local-IPv4 lookup used by registration and
startup logs. Not a networking
framework — no interface enumeration, CIDR matching, or address parsing
beyond `net` itself.

## Usage

```go
import "go-spring.org/stdlib/netutil"

ip := netutil.LocalIPv4()
```

### API

- `LocalIPv4() string` — the first non-loopback IPv4 address of the local
  machine, or `"0.0.0.0"` when none is available. Cached after the first
  call.

## License

Apache License 2.0. See [LICENSE](../../LICENSE).
