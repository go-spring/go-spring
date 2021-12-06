# apcu

提供了进程内缓存组件。

## Install

```
go get github.com/go-spring/spring-base@v1.1.0-rc2 
```

## Import

```
import "github.com/go-spring/spring-base/apcu"
```

## Example

```
var i int
load, err := apcu.Load(ctx, "int", &i)
apcu.Store(ctx, "int", 3)
load, err := apcu.Load(ctx, "int", &i)
apcu.Delete(ctx, "int")
```