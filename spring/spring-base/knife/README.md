# knife

Provides cache on `context.Context`.

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
err = knife.Store(ctx, "a", "b")
v, err := knife.Load(ctx, "a")
v, err = knife.LoadOrStore(ctx, "a", "b")
```