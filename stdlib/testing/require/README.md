# require
[English](README.md) | [中文](README_CN.md)

`require` provides fluent, type-specific assertions that **stop the test on
the first failure** — no further assertions run. The sibling package
[`assert`](../assert/) has identical semantics but continues on failure.

See the parent package [`testing`](../) for the full assertion reference and
comparison between `assert` and `require`.

## When to use `require` over `assert`

Use `require` when a failed check invalidates everything after it — e.g. the
value being unwrapped is nil, or the fixture failed to set up. Continuing
would either panic or produce nonsensical failure output. Use `assert` when
multiple independent assertions in one test are informative on their own.

## Features

- Same fluent API as `assert`: `That`, `Error`, `Number[T]`, `String`,
  `Slice[T]`, `Map[K,V]`, top-level `Panic`.
- Fails with `t.Fatalf` — the test stops immediately.
- Every method accepts trailing `msg ...string` for custom failure messages.
- Zero third-party dependencies.

## Usage

```go
package myapp_test

import (
    "testing"

    "go-spring.org/stdlib/testing/assert"
    "go-spring.org/stdlib/testing/require"
)

func TestUser(t *testing.T) {
    user := loadUser()
    require.That(t, user).NotNil() // stop here if nil — the next line would panic

    assert.String(t, user.Email).IsEmail()
    assert.Number(t, user.Age).GreaterThan(0)
}
```
