# knife

提供了 context.Context 上的缓存。

## Install

```
go get github.com/go-spring/spring-base@v1.1.0-rc2 
```

## Import

```
import "github.com/go-spring/spring-base/knife"
```

## Example

```
ctx = knife.New(context.Background())

err = knife.Set(ctx, "a", "b")
v, ok = knife.Get(ctx, "a")

var m map[string]string
ok, err := knife.Fetch(ctx, "a", &m)
```