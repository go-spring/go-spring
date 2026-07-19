# hashutil
[English](README.md) | [中文](README_CN.md)

`hashutil` is a thin convenience wrapper around `hash/fnv`. Part of
Go-Spring's zero-dependency `stdlib` layer.

## API

- `FNV1a64(s string) uint64` — 64-bit FNV-1a of a string, using the standard
  library `hash/fnv` implementation.

## Usage

```go
import "go-spring.org/stdlib/hashutil"

h := hashutil.FNV1a64("some/key")
```

FNV-1a is a fast, non-cryptographic hash. Suitable for map sharding, cache
bucketing, and similar tasks. Do not use it where an adversary can choose
inputs.
