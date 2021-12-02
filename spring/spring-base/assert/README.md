# assert

提供了一些常用的断言函数。

## Install

```
go get github.com/go-spring/spring-base@v1.1.0-rc2 
```

## Import

```
import "github.com/go-spring/spring-base/assert"
```

## Example

```
assert.True(t, true)
assert.Nil(t, nil)
assert.Equal(t, 0, "0")
assert.NotEqual(t, "0", 0)
assert.Same(t, 0, "0")
assert.NotSame(t, "0", "0")
assert.Panic(g, func() {}, "an error")
assert.Matches(g, "there's no error", "an error")
assert.Error(g, errors.New("there's no error"), "an error")
assert.TypeOf(g, new(int), (*int)(nil))
assert.Implements(g, errors.New("error"), (*error)(nil))
```
