# spring-logger

该项目定义了一个标准的日志输出接口 `StdLogger` 和一个封装了 context.Context 对象的日志输出接口 `ContextLogger`，并分别提供了默认实现。

<br>

# 目录

- [Level](#level)
- [StdLogger](#stdlogger)
- [DefaultLogger](#defaultlogger)
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

    // SetLevel 设置日志的输出级别，请确保线程安全
	SetLevel(level Level)

	// 输出 TRACE 级别的日志
	Trace(args ...interface{})
	Tracef(format string, args ...interface{})

	// 输出 DEBUG 级别的日志
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})

	// 输出 INFO 级别的日志
	Info(args ...interface{})
	Infof(format string, args ...interface{})

	// 输出 WARN 级别的日志
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})

	// 输出 ERROR 级别的日志
	Error(args ...interface{})
	Errorf(format string, args ...interface{})

	// 输出 PANIC 级别的日志
	Panic(args ...interface{})
	Panicf(format string, args ...interface{})

	// 输出 FATAL 级别的日志
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})

	// 将日志内容输出到控制台
	Print(args ...interface{})
	Printf(format string, args ...interface{})

	// 输出自定义级别的日志，skip 是相对于当前函数的调用深度
	Output(skip int, level Level, args ...interface{})
	Outputf(skip int, level Level, format string, args ...interface{})

### 全局函数

	// SetLogger 设置新的日志输出器
	func SetLogger(logger StdLogger)

	// SetLevel 设置日志的输出级别，请确保线程安全
	func SetLevel(level Level)

	// Trace 输出 TRACE 级别的日志
	func Trace(args ...interface{})
	func Tracef(format string, args ...interface{})

	// Debug 输出 DEBUG 级别的日志
	func Debug(args ...interface{})
	func Debugf(format string, args ...interface{})

	// Info 输出 INFO 级别的日志
	func Info(args ...interface{})
	func Infof(format string, args ...interface{})

	// Warn 输出 WARN 级别的日志
	func Warn(args ...interface{})
	func Warnf(format string, args ...interface{})

	// Error 输出 ERROR 级别的日志
	func Error(args ...interface{})
	func Errorf(format string, args ...interface{})

	// Panic 输出 PANIC 级别的日志
	func Panic(args ...interface{})
	func Panicf(format string, args ...interface{})

	// Fatal 输出 FATAL 级别的日志
	func Fatal(args ...interface{})
	func Fatalf(format string, args ...interface{})

	// Output 自定义日志级别和调用栈深度，skip 是相对于当前函数的调用深度
	func Output(skip int, level Level, args ...interface{})
	func Outputf(skip int, level Level, format string, args ...interface{})

#### Console

实现了 StdLogger 接口。

    // NewConsole Console 的构造函数
    func NewConsole(level Level) *Console

### ContextLogger

封装了 context.Context 对象的日志输出接口。

	// Logger 返回封装了 context.Context 和自定义标签的 StdLogger 对象
	Logger(tags ...string) StdLogger

	// 输出 TRACE 级别的日志
	LogTrace(args ...interface{})
	LogTracef(format string, args ...interface{})

	// 输出 DEBUG 级别的日志
	LogDebug(args ...interface{})
	LogDebugf(format string, args ...interface{})

	// 输出 INFO 级别的日志
	LogInfo(args ...interface{})
	LogInfof(format string, args ...interface{})

	// 输出 WARN 级别的日志
	LogWarn(args ...interface{})
	LogWarnf(format string, args ...interface{})

	// 输出 ERROR 级别的日志
	LogError(args ...interface{})
	LogErrorf(format string, args ...interface{})

	// 输出 PANIC 级别的日志
	LogPanic(args ...interface{})
	LogPanicf(format string, args ...interface{})

	// 输出 FATAL 级别的日志
	LogFatal(args ...interface{})
	LogFatalf(format string, args ...interface{})

#### DefaultContextLogger

ContextLogger 的默认实现。

	// NewDefaultContextLogger DefaultContextLogger 的构造函数
	func NewDefaultContextLogger(ctx context.Context) *DefaultContextLogger