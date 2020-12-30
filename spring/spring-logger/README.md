# spring-logger

该包定义了一个标准的日志输出接口 `StdLogger`，并提供了输出到控制台的实现 `Console`。另外，该包也提供了一个封装了 `context.Context` 和自定义标签的 `ContextLogger`，可以满足基于 `context.Context` 和自定义标签的日志输出。

# 示例

```
SpringLogger.SetLogger(SpringLogger.NewConsole(SpringLogger.InfoLevel))

SpringLogger.Trace("a", "=", "1")
SpringLogger.Tracef("a=%d", 1)

SpringLogger.Debug("a", "=", "1")
SpringLogger.Debugf("a=%d", 1)

SpringLogger.Info("a", "=", "1")
SpringLogger.Infof("a=%d", 1)

SpringLogger.Warn("a", "=", "1")
SpringLogger.Warnf("a=%d", 1)

SpringLogger.Error("a", "=", "1")
SpringLogger.Errorf("a=%d", 1)
```

```
SpringLogger.RegisterContextOutput(&NativeLogger{})
ctx := context.WithValue(context.TODO(), "trace_id", "0689")

logger := SpringLogger.WithContext(ctx)
logger.Trace("level:", "trace")
logger.Tracef("level:%s", "trace")
logger.Debug("level:", "debug")
logger.Debugf("level:%s", "debug")
logger.Info("level:", "info")
logger.Infof("level:%s", "info")
logger.Warn("level:", "warn")
logger.Warnf("level:%s", "warn")
logger.Error("level:", "error")
logger.Errorf("level:%s", "error")
logger.Panic("level:", "panic")
logger.Panicf("level:%s", "panic")
logger.Fatal("level:", "fatal")
logger.Fatalf("level:%s", "fatal")

logger.WithTag("__in").Trace("level:", "trace")
logger.WithTag("__in").Tracef("level:%s", "trace")
logger.WithTag("__in").Debug("level:", "debug")
logger.WithTag("__in").Debugf("level:%s", "debug")
logger.WithTag("__in").Info("level:", "info")
logger.WithTag("__in").Infof("level:%s", "info")
logger.WithTag("__in").Warn("level:", "warn")
logger.WithTag("__in").Warnf("level:%s", "warn")
logger.WithTag("__in").Error("level:", "error")
logger.WithTag("__in").Errorf("level:%s", "error")
logger.WithTag("__in").Panic("level:", "panic")
logger.WithTag("__in").Panicf("level:%s", "panic")
logger.WithTag("__in").Fatal("level:", "fatal")
logger.WithTag("__in").Fatalf("level:%s", "fatal")
```

# API

- [Level](#level)
- [StdLogger](#stdlogger)
    - [全局函数](#全局函数)
    - [Console](#console)
    - [ContextLogger](#contextlogger)

### Level

日志级别。

	TraceLevel
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	PanicLevel
	FatalLevel

### StdLogger

标准日志输出接口。

	// 输出 TRACE 级别的日志。
	Trace(args ...interface{})
	Tracef(format string, args ...interface{})

	// 输出 DEBUG 级别的日志。
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})

	// 输出 INFO 级别的日志。
	Info(args ...interface{})
	Infof(format string, args ...interface{})

	// 输出 WARN 级别的日志。
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})

	// 输出 ERROR 级别的日志。
	Error(args ...interface{})
	Errorf(format string, args ...interface{})

	// 输出 PANIC 级别的日志。
	Panic(args ...interface{})
	Panicf(format string, args ...interface{})

	// 输出 FATAL 级别的日志。
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})

	// 将日志内容输出到控制台。
	Print(args ...interface{})
	Printf(format string, args ...interface{})

	// 输出自定义级别的日志，skip 是相对于当前函数的调用深度。
	Output(skip int, level Level, args ...interface{})
	Outputf(skip int, level Level, format string, args ...interface{})

#### 全局函数

	// SetLogger 设置新的日志输出器。
	func SetLogger(logger StdLogger)

	// Trace 输出 TRACE 级别的日志。
	func Trace(args ...interface{})
	func Tracef(format string, args ...interface{})

	// Debug 输出 DEBUG 级别的日志。
	func Debug(args ...interface{})
	func Debugf(format string, args ...interface{})

	// Info 输出 INFO 级别的日志。
	func Info(args ...interface{})
	func Infof(format string, args ...interface{})

	// Warn 输出 WARN 级别的日志。
	func Warn(args ...interface{})
	func Warnf(format string, args ...interface{})

	// Error 输出 ERROR 级别的日志。
	func Error(args ...interface{})
	func Errorf(format string, args ...interface{})

	// Panic 输出 PANIC 级别的日志。
	func Panic(args ...interface{})
	func Panicf(format string, args ...interface{})

	// Fatal 输出 FATAL 级别的日志。
	func Fatal(args ...interface{})
	func Fatalf(format string, args ...interface{})

#### Console

输出到控制台的 `StdLogger` 实现。

    // NewConsole Console 的构造函数。
    func NewConsole(level Level) *Console

#### ContextLogger

封装了 `context.Context` 和自定义标签的 `StdLogger` 对象。

	// WithContext ContextLogger 的构造函数，自定义标签可以为空。
	func WithContext(ctx context.Context, tag ...string) *ContextLogger 