/*
 * Copyright 2012-2019 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package SpringLogger

const (
	TraceLevel Level = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	PanicLevel
	FatalLevel
)

// Level 日志输出级别
type Level uint32

func (l Level) String() string {
	switch l {
	case TraceLevel:
		return "trace"
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case PanicLevel:
		return "panic"
	case FatalLevel:
		return "fatal"
	}
	return ""
}

// StdLogger 标准日志输出接口
type StdLogger interface {

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
}

// StdLoggerWrapper 平衡调用栈的深度
type StdLoggerWrapper struct {
	StdLogger
}

func (w *StdLoggerWrapper) SetLevel(level Level) {
	w.StdLogger.SetLevel(level)
}

func (w *StdLoggerWrapper) Trace(args ...interface{}) {
	w.StdLogger.Output(1, TraceLevel, args...)
}

func (w *StdLoggerWrapper) Tracef(format string, args ...interface{}) {
	w.StdLogger.Outputf(1, TraceLevel, format, args...)
}

func (w *StdLoggerWrapper) Debug(args ...interface{}) {
	w.StdLogger.Output(1, DebugLevel, args...)
}

func (w *StdLoggerWrapper) Debugf(format string, args ...interface{}) {
	w.StdLogger.Outputf(1, DebugLevel, format, args...)
}

func (w *StdLoggerWrapper) Info(args ...interface{}) {
	w.StdLogger.Output(1, InfoLevel, args...)
}

func (w *StdLoggerWrapper) Infof(format string, args ...interface{}) {
	w.StdLogger.Outputf(1, InfoLevel, format, args...)
}

func (w *StdLoggerWrapper) Warn(args ...interface{}) {
	w.StdLogger.Output(1, WarnLevel, args...)
}

func (w *StdLoggerWrapper) Warnf(format string, args ...interface{}) {
	w.StdLogger.Outputf(1, WarnLevel, format, args...)
}

func (w *StdLoggerWrapper) Error(args ...interface{}) {
	w.StdLogger.Output(1, ErrorLevel, args...)
}

func (w *StdLoggerWrapper) Errorf(format string, args ...interface{}) {
	w.StdLogger.Outputf(1, ErrorLevel, format, args...)
}

func (w *StdLoggerWrapper) Panic(args ...interface{}) {
	w.StdLogger.Output(1, PanicLevel, args...)
}

func (w *StdLoggerWrapper) Panicf(format string, args ...interface{}) {
	w.StdLogger.Outputf(1, PanicLevel, format, args...)
}

func (w *StdLoggerWrapper) Fatal(args ...interface{}) {
	w.StdLogger.Output(1, FatalLevel, args...)
}

func (w *StdLoggerWrapper) Fatalf(format string, args ...interface{}) {
	w.StdLogger.Outputf(1, FatalLevel, format, args...)
}

func (w *StdLoggerWrapper) Print(args ...interface{}) {
	w.StdLogger.Print(args...)
}

func (w *StdLoggerWrapper) Printf(format string, args ...interface{}) {
	w.StdLogger.Printf(format, args...)
}

// Output 输出自定义级别的日志，skip 是相对于当前函数的调用深度
func (w *StdLoggerWrapper) Output(skip int, level Level, args ...interface{}) {
	w.StdLogger.Output(skip+1, level, args...)
}

// Outputf 输出自定义级别的日志，skip 是相对于当前函数的调用深度
func (w *StdLoggerWrapper) Outputf(skip int, level Level, format string, args ...interface{}) {
	w.StdLogger.Outputf(skip+1, level, format, args...)
}
