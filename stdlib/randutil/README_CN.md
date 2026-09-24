# randutil

[English](README.md) | [中文](README_CN.md)

`randutil` 生成随机标识符字符串——fencing token、request id、session id、CSRF
token、OAuth2 不透明值这些，不抽取的话会在代码库里到处都是手写的
`crypto/rand` 片段。

## Hex 与 URLSafe

- `Hex(n)`——n 个随机字节的 2n 个 hex 字符；`n=16` 是常用短 id 尺寸（128 位）。
- `URLSafe(n)`——n 个随机字节的无填充 base64url；`n=32`（256 位）是常用
  不可猜测 token 尺寸。

Go 1.24 起 `crypto/rand` 不会失败，这两个函数也不会。
