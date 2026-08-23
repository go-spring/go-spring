# md5util

[English](README.md) | [中文](README_CN.md)

`MD5` computes the MD5 checksum of a string and returns it as a lowercase hex
string.

## Usage

```go
import "go-spring.org/stdlib/md5util"

sum := md5util.MD5("hello") // "5d41402abc4b2a76b9719d911017c592"
```

### API

- `MD5(str string) string` — lowercase hex-encoded MD5 digest.

MD5 is **not** suitable for cryptographic authentication. Use it only for
checksums, cache keys, or fingerprints where collisions are tolerable.

## License

Apache License 2.0. See [LICENSE](../../LICENSE).
