# spring-logger

@[toc]

### *_Level_*

	TraceLevel
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	PanicLevel
	FatalLevel

### *_StdLogger_*

    // SetLevel 设置日志的输出级别，请确保线程安全
	SetLevel(level Level)

	Trace(args ...interface{})
	Tracef(format string, args ...interface{})

	Debug(args ...interface{})
	Debugf(format string, args ...interface{})

	Info(args ...interface{})
	Infof(format string, args ...interface{})

	Warn(args ...interface{})
	Warnf(format string, args ...interface{})

	Error(args ...interface{})
	Errorf(format string, args ...interface{})

	Panic(args ...interface{})
	Panicf(format string, args ...interface{})

	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})

	Print(args ...interface{})
	Printf(format string, args ...interface{})

	// skip 是相对于 Output & Outputf 的调用栈深度
	Output(skip int, level Level, args ...interface{})
	Outputf(skip int, level Level, format string, args ...interface{})

### *_DefaultLogger_*

    // SetLogger 设置新的日志输出器
    func SetLogger(logger StdLogger)
    
    // SetLevel 设置日志的输出级别，线程安全
    func SetLevel(level Level)
    
    // Trace 打印 TRACE 日志
    func Trace(args ...interface{})
    func Tracef(format string, args ...interface{})
    
    // Debug 打印 DEBUG 日志
    func Debug(args ...interface{})
    func Debugf(format string, args ...interface{})
    
    // Info 打印 INFO 日志
    func Info(args ...interface{})
    func Infof(format string, args ...interface{})
    
    // Warn 打印 WARN 日志
    func Warn(args ...interface{})
    func Warnf(format string, args ...interface{})
    
    // Error 打印 ERROR 日志
    func Error(args ...interface{})
    func Errorf(format string, args ...interface{})
    
    // Panic 打印 PANIC 日志
    func Panic(args ...interface{})
    func Panicf(format string, args ...interface{})
    
    // Fatal 打印 FATAL 日志
    func Fatal(args ...interface{})
    func Fatalf(format string, args ...interface{})

    // Output 自定义日志级别和调用栈深度，skip 是相对于 Output 的调用栈深度
    func Output(skip int, level Level, args ...interface{})
    func Outputf(skip int, level Level, format string, args ...interface{})

#### Console

    // NewConsole Console 的构造函数
    func NewConsole(level Level) *Console

### *_PrefixLogger_*

	LogTrace(args ...interface{})
	LogTracef(format string, args ...interface{})

	LogDebug(args ...interface{})
	LogDebugf(format string, args ...interface{})

	LogInfo(args ...interface{})
	LogInfof(format string, args ...interface{})

	LogWarn(args ...interface{})
	LogWarnf(format string, args ...interface{})

	LogError(args ...interface{})
	LogErrorf(format string, args ...interface{})

	LogPanic(args ...interface{})
	LogPanicf(format string, args ...interface{})

	LogFatal(args ...interface{})
	LogFatalf(format string, args ...interface{})

### *_LoggerContext_*

	// PrefixLogger 带有前缀的 Logger 接口
	PrefixLogger

	// Logger 获取标准 Logger 接口
	Logger(tags ...string) StdLogger

#### DefaultLoggerContext

    // NewDefaultLoggerContext DefaultLoggerContext 的构造函数
    func NewDefaultLoggerContext(ctx context.Context) *DefaultLoggerContext