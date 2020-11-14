# spring-logger

该包定义了一个标准的日志输出接口 `StdLogger` 和一个封装了 `context.Context`
对象的日志输出接口 `ContextLogger`，并分别提供了默认实现 `Console` 和
`DefaultContextLogger`。

# 示例

```
SpringLogger.SetLogger(SpringLogger.NewConsole(SpringLogger.InfoLevel))
SpringLogger.SetLevel(SpringLogger.TraceLevel)

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

func() {
    defer func() { fmt.Println(recover()) }()
    SpringLogger.Panic("error")
}()

func() {
    defer func() { fmt.Println(recover()) }()
    SpringLogger.Panic(errors.New("error"))
}()

func() {
    defer func() { fmt.Println(recover()) }()
    SpringLogger.Panicf("error: %d", 404)
}()

// SpringLogger.Fatal("a", "=", "1")
// SpringLogger.Fatalf("a=%d", 1)

SpringLogger.Output(0, SpringLogger.InfoLevel, "a=1")
SpringLogger.Outputf(0, SpringLogger.InfoLevel, "a=%d", 1)
```

```
// 设置全局转换函数。
SpringLogger.Logger = func(ctx context.Context, tags ...string) SpringLogger.StdLogger {
    return &ContextLogger{ctx: ctx, tags: tags}
}

ctx := context.WithValue(context.TODO(), "trace_id", "0689")
tracer := SpringLogger.NewDefaultContextLogger(ctx)

tracer.LogTrace("level:", "trace")
tracer.LogTracef("level:%s", "trace")
tracer.LogDebug("level:", "debug")
tracer.LogDebugf("level:%s", "debug")
tracer.LogInfo("level:", "info")
tracer.LogInfof("level:%s", "info")
tracer.LogWarn("level:", "warn")
tracer.LogWarnf("level:%s", "warn")
tracer.LogError("level:", "error")
tracer.LogErrorf("level:%s", "error")
tracer.LogPanic("level:", "panic")
tracer.LogPanicf("level:%s", "panic")
tracer.LogFatal("level:", "fatal")
tracer.LogFatalf("level:%s", "fatal")

tracer.Logger("__in").Trace("level:", "trace")
tracer.Logger("__in").Tracef("level:%s", "trace")
tracer.Logger("__in").Debug("level:", "debug")
tracer.Logger("__in").Debugf("level:%s", "debug")
tracer.Logger("__in").Info("level:", "info")
tracer.Logger("__in").Infof("level:%s", "info")
tracer.Logger("__in").Warn("level:", "warn")
tracer.Logger("__in").Warnf("level:%s", "warn")
tracer.Logger("__in").Error("level:", "error")
tracer.Logger("__in").Errorf("level:%s", "error")
tracer.Logger("__in").Panic("level:", "panic")
tracer.Logger("__in").Panicf("level:%s", "panic")
tracer.Logger("__in").Fatal("level:", "fatal")
tracer.Logger("__in").Fatalf("level:%s", "fatal")
```

# API

- [Level](#level)
- [StdLogger](#stdlogger)
- [全局函数](#全局函数)
	- [Console](#console)
- [ContextLogger](#contextlogger)
	- [DefaultContextLogger](#defaultcontextlogger)


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

    // SetLevel 设置日志的输出级别，请确保线程安全。
	SetLevel(level Level)

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

### 全局函数

	// SetLogger 设置新的日志输出器。
	func SetLogger(logger StdLogger)

	// SetLevel 设置日志的输出级别，请确保线程安全。
	func SetLevel(level Level)

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

	// Output 自定义日志级别和调用栈深度，skip 是相对于当前函数的调用深度。
	func Output(skip int, level Level, args ...interface{})
	func Outputf(skip int, level Level, format string, args ...interface{})

#### Console

实现了 StdLogger 接口。

    // NewConsole Console 的构造函数。
    func NewConsole(level Level) *Console

### ContextLogger

封装了 context.Context 对象的日志输出接口。

	// Logger 返回封装了 context.Context 和自定义标签的 StdLogger 对象。
	Logger(tags ...string) StdLogger

	// 输出 TRACE 级别的日志。
	LogTrace(args ...interface{})
	LogTracef(format string, args ...interface{})

	// 输出 DEBUG 级别的日志。
	LogDebug(args ...interface{})
	LogDebugf(format string, args ...interface{})

	// 输出 INFO 级别的日志。
	LogInfo(args ...interface{})
	LogInfof(format string, args ...interface{})

	// 输出 WARN 级别的日志。
	LogWarn(args ...interface{})
	LogWarnf(format string, args ...interface{})

	// 输出 ERROR 级别的日志。
	LogError(args ...interface{})
	LogErrorf(format string, args ...interface{})

	// 输出 PANIC 级别的日志。
	LogPanic(args ...interface{})
	LogPanicf(format string, args ...interface{})

	// 输出 FATAL 级别的日志。
	LogFatal(args ...interface{})
	LogFatalf(format string, args ...interface{})

#### DefaultContextLogger

ContextLogger 的默认实现。

	// NewDefaultContextLogger DefaultContextLogger 的构造函数。
	func NewDefaultContextLogger(ctx context.Context) *DefaultContextLogger