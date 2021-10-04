# log

重新定义标准日志接口，可以灵活适配各种日志框架。

## 日志级别

```go
const (
	TraceLevel = Level(iota)
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	PanicLevel
	FatalLevel
)
```

## 日志配置

```go
var config = struct {
	mutex  sync.Mutex
	level  Level
	output Output
}
```

### 恢复默认的输出配置

```go
func Reset()
```

### 设置日志的输出级别

```go
func SetLevel(level Level)
```

### 设置日志的输出格式

```go
func SetOutput(output Output)
```

## 标准输出

```go
func Trace(args ...interface{})
func Tracef(format string, args ...interface{})
func Debug(args ...interface{})
func Debugf(format string, args ...interface{})
func Info(args ...interface{})
func Infof(format string, args ...interface{})
func Warn(args ...interface{})
func Warnf(format string, args ...interface{})
func Error(args ...interface{})
func Errorf(format string, args ...interface{})
func Panic(args ...interface{})
func Panicf(format string, args ...interface{})
func Fatal(args ...interface{})
func Fatalf(format string, args ...interface{})
```

## 自定义输出

```go
func Tag(tag string) Entry
func Ctx(ctx context.Context) Entry
func (e Entry) Trace(args ...interface{})
func (e Entry) Tracef(format string, args ...interface{})
func (e Entry) Debug(args ...interface{})
func (e Entry) Debugf(format string, args ...interface{})
func (e Entry) Info(args ...interface{})
func (e Entry) Infof(format string, args ...interface{})
func (e Entry) Warn(args ...interface{})
func (e Entry) Warnf(format string, args ...interface{})
func (e Entry) Error(args ...interface{})
func (e Entry) Errorf(format string, args ...interface{})
func (e Entry) Panic(args ...interface{})
func (e Entry) Panicf(format string, args ...interface{})
func (e Entry) Fatal(args ...interface{})
func (e Entry) Fatalf(format string, args ...interface{})
```

## 自定义输出格式

```go
type Output func(level Level, e *Entry)
```

```go
func Console(level Level, e *Entry) {
	strLevel := strings.ToUpper(level.String())
	if level >= ErrorLevel {
		strLevel = console.Red.Sprint(strLevel)
	} else if level == WarnLevel {
		strLevel = console.Yellow.Sprint(strLevel)
	} else if level == TraceLevel {
		strLevel = console.Green.Sprint(strLevel)
	}
	_, _ = fmt.Printf("[%s] %s:%d %s\n", strLevel, e.file, e.line, e.msg)
}
```

## 参数延迟计算

```go
log.Trace(func() []interface{} {
    return log.T("a", "=", "1")
})
```

```go
log.Tracef("a=%d", func() []interface{} {
    return log.T(1)
})
```
