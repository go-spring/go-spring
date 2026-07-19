# require
[English](README.md) | [中文](README_CN.md)

`require` 提供 fluent、按类型分族的断言,**首次失败即停**——后续断言不再
执行。姊妹包 [`assert`](../assert/) 语义一致但失败继续。

完整断言参考与 `assert` / `require` 对比,见父包 [`testing`](../)。

## 何时选 `require` 而不是 `assert`

当一次失败会让后面的检查全部无意义时用 `require`——如被 unwrap 的值是 nil,
或 fixture 没装配成功。继续下去要么 panic,要么打出胡言乱语的错误。多个独
立断言各自有意义时用 `assert`。

## 特性

- 与 `assert` 完全同款 fluent API:`That` / `Error` / `Number[T]` /
  `String` / `Slice[T]` / `Map[K,V]` / 顶层 `Panic`。
- 失败走 `t.Fatalf`——测试立即停止。
- 所有方法末尾接受 `msg ...string` 自定义失败信息。
- 零第三方依赖。

## 用法

```go
package myapp_test

import (
    "testing"

    "go-spring.org/stdlib/testing/assert"
    "go-spring.org/stdlib/testing/require"
)

func TestUser(t *testing.T) {
    user := loadUser()
    require.That(t, user).NotNil() // nil 就停——不然下一行 panic

    assert.String(t, user.Email).IsEmail()
    assert.Number(t, user.Age).GreaterThan(0)
}
```
