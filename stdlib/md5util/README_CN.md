# md5util

[English](README.md) | [中文](README_CN.md)

`MD5` 计算字符串的 MD5 摘要，并以小写 hex 字符串返回。

## 使用方式

```go
import "go-spring.org/stdlib/md5util"

sum := md5util.MD5("hello") // "5d41402abc4b2a76b9719d911017c592"
```

### API 列表

- `MD5(str string) string` —— 小写 hex 编码的 MD5 摘要。

MD5 **不适合**作为密码学认证，仅用于校验和、缓存 key、指纹等允许碰撞的场景。

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。
