# assert

Provides some useful assertion methods.

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
assert.Panic(t, func() {}, "an error")
assert.Matches(t, "there's no error", "an error")
assert.Error(t, errors.New("there's no error"), "an error")
assert.TypeOf(t, new(int), (*int)(nil))
assert.Implements(t, errors.New("error"), (*error)(nil))
```
