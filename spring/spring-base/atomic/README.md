# atomic

Provides simple wrappers for `sync/atomic` to enforce atomic access.

## Install

```
go get github.com/go-spring/spring-base@v1.1.0-rc2 
```

## Import

```
import "github.com/go-spring/spring-base/atomic"
```

## Example

```
var i atomic.Int64
i.Add(1)
i.Store(2)
_ = i.Load()
```