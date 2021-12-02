# log

重新定义标准日志接口，可以灵活适配各种日志框架。

## Install

```
go get github.com/go-spring/spring-base@v1.1.0-rc2 
```

## Import

```
import "github.com/go-spring/spring-base/log"
```

## Example

```
log.SetLevel(log.TraceLevel)
defer log.Reset()

log.Trace("a", "=", "1")
log.Tracef("a=%d", 1)

log.Trace(func() []interface{} {
    return log.T("a", "=", "1")
})

log.Tracef("a=%d", func() []interface{} {
    return log.T(1)
})

...
```

```
ctx := context.WithValue(context.TODO(), traceIDKey, "0689")

log.SetLevel(log.TraceLevel)
log.SetOutput(myOutput)
defer log.Reset()

logger := log.Ctx(ctx)
logger.Trace("level:", "trace")
logger.Tracef("level:%s", "trace")

...

logger = log.Tag("__in")
logger.Ctx(ctx).Trace("level:", "trace")
logger.Ctx(ctx).Tracef("level:%s", "trace")

...
```