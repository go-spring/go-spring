# assert
[English](README.md) | [中文](README_CN.md)

`assert` provides fluent, type-specific assertions that **do not stop the
test on failure** — subsequent assertions still run so a single test can
report multiple issues at once. The sibling package [`require`](../require/)
has identical semantics but stops on the first failure.

See the parent package [`testing`](../) for the full assertion reference and
comparison between `assert` and `require`.

## Features

- Generic entry points: `That`, `Error`, `Number[T]`, `String`, `Slice[T]`,
  `Map[K,V]`, top-level `Panic`.
- Fluent chained checks (e.g. `.Equal(...)`, `.NotNil()`, `.Contains(...)`).
- Every method accepts trailing `msg ...string` for custom failure messages.
- Zero third-party dependencies.

## Usage

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

    // Failure here does not stop the test — the next assertion still runs.
    assert.That(t, "a").Equal("b")
    assert.That(t, "c").Equal("c") // will still execute
}
```
