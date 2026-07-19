# assert
[English](README.md) | [中文](README_CN.md)

`assert` 提供 fluent、按类型分族的断言,失败**不停测试**——后续断言继续
跑,一次测试可以报出多个问题。姊妹包 [`require`](../require/) 语义一致但
首次失败即停。

完整断言参考与 `assert` / `require` 对比,见父包 [`testing`](../)。

## 特性

- 泛型入口:`That` / `Error` / `Number[T]` / `String` / `Slice[T]` /
  `Map[K,V]` / 顶层 `Panic`。
- Fluent 链式检查(如 `.Equal(...)`、`.NotNil()`、`.Contains(...)`)。
- 所有方法末尾接受 `msg ...string` 传自定义失败信息。
- 零第三方依赖。

## 用法

```go
package myapp_test

import (
    "testing"

    "go-spring.org/stdlib/testing/assert"
)

func TestUser(t *testing.T) {
    assert.That(t, "hello").Equal("hello")
    assert.Number(t, 42).GreaterThan(40)
    assert.String(t, "user@example.com").IsEmail()
    assert.Slice(t, []int{1, 2, 3}).Contains(2)

    // 这里失败不会停测——下一条断言依然会执行。
    assert.That(t, "a").Equal("b")
    assert.That(t, "c").Equal("c") // 仍然会执行
}
```
