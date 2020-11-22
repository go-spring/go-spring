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

// 该包定义了一个标准的日志输出接口 StdLogger，并提供了输出到控制台的实现
// Console。另外，该包也提供了一个封装了 context.Context 和自定义标签的
// ContextLogger 类型，可以满足基于 context.Context 的日志输出。
package SpringLogger

import (
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/labstack/gommon/color"
)

const (
	TraceLevel Level = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	PanicLevel
	FatalLevel
)

// Level 日志输出级别。
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

// StdLogger 标准日志输出接口。
type StdLogger interface {

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
}

// OutputFunc 为 Console 定制日志输出格式，skip 是相对于当前函数的调用深度。
type OutputFunc func(skip int, level Level, msg string)

// Console 将日志输出到控制台，StdLogger 的默认实现。
type Console struct {
	level  Level
	output OutputFunc
}

// NewConsole Console 的构造函数。
func NewConsole(level Level, output ...OutputFunc) *Console {
	if len(output) > 0 {
		if output[0] == nil {
			panic(errors.New("output[0] is <nil>"))
		}
		return &Console{level: level, output: output[0]}
	}
	return &Console{level: level, output: func(skip int, level Level, msg string) {

		strLevel := strings.ToUpper(level.String())
		if level >= ErrorLevel { // TODO 去掉颜色依赖
			strLevel = color.Red(strLevel)
		} else if level == WarnLevel {
			strLevel = color.Yellow(strLevel)
		}

		_, file, line, _ := runtime.Caller(skip + 1)
		dir, filename := path.Split(file)
		filename = path.Join(path.Base(dir), filename)
		fmt.Fprintf(os.Stdout, "[%s] %s:%d %s\n", strLevel, filename, line, msg)

		switch level {
		case PanicLevel:
			panic(msg)
		case FatalLevel:
			os.Exit(1)
		}
	}}
}

// Trace 输出 TRACE 级别的日志。
func (c *Console) Trace(args ...interface{}) {
	if c.level <= TraceLevel {
		c.output(1, TraceLevel, fmt.Sprint(args...))
	}
}

// Tracef 输出 TRACE 级别的日志。
func (c *Console) Tracef(format string, args ...interface{}) {
	if c.level <= TraceLevel {
		c.output(1, TraceLevel, fmt.Sprintf(format, args...))
	}
}

// Debug 输出 DEBUG 级别的日志。
func (c *Console) Debug(args ...interface{}) {
	if c.level <= DebugLevel {
		c.output(1, DebugLevel, fmt.Sprint(args...))
	}
}

// Debugf 输出 DEBUG 级别的日志。
func (c *Console) Debugf(format string, args ...interface{}) {
	if c.level <= DebugLevel {
		c.output(1, DebugLevel, fmt.Sprintf(format, args...))
	}
}

// Info 输出 INFO 级别的日志。
func (c *Console) Info(args ...interface{}) {
	if c.level <= InfoLevel {
		c.output(1, InfoLevel, fmt.Sprint(args...))
	}
}

// Infof 输出 INFO 级别的日志。
func (c *Console) Infof(format string, args ...interface{}) {
	if c.level <= InfoLevel {
		c.output(1, InfoLevel, fmt.Sprintf(format, args...))
	}
}

// Warn 输出 WARN 级别的日志。
func (c *Console) Warn(args ...interface{}) {
	if c.level <= WarnLevel {
		c.output(1, WarnLevel, fmt.Sprint(args...))
	}
}

// Warnf 输出 WARN 级别的日志。
func (c *Console) Warnf(format string, args ...interface{}) {
	if c.level <= WarnLevel {
		c.output(1, WarnLevel, fmt.Sprintf(format, args...))
	}
}

// Error 输出 ERROR 级别的日志。
func (c *Console) Error(args ...interface{}) {
	if c.level <= ErrorLevel {
		c.output(1, ErrorLevel, fmt.Sprint(args...))
	}
}

// Errorf 输出 ERROR 级别的日志。
func (c *Console) Errorf(format string, args ...interface{}) {
	if c.level <= ErrorLevel {
		c.output(1, ErrorLevel, fmt.Sprintf(format, args...))
	}
}

// Panic 输出 PANIC 级别的日志。
func (c *Console) Panic(args ...interface{}) {
	if c.level <= PanicLevel {
		c.output(1, PanicLevel, fmt.Sprint(args...))
	}
}

// Panicf 输出 PANIC 级别的日志。
func (c *Console) Panicf(format string, args ...interface{}) {
	if c.level <= PanicLevel {
		c.output(1, PanicLevel, fmt.Sprintf(format, args...))
	}
}

// Fatal 输出 FATAL 级别的日志。
func (c *Console) Fatal(args ...interface{}) {
	if c.level <= FatalLevel {
		c.output(1, FatalLevel, fmt.Sprint(args...))
	}
}

// Fatalf 输出 FATAL 级别的日志。
func (c *Console) Fatalf(format string, args ...interface{}) {
	if c.level <= FatalLevel {
		c.output(1, FatalLevel, fmt.Sprintf(format, args...))
	}
}

// Print 将日志内容输出到控制台。
func (c *Console) Print(args ...interface{}) {
	fmt.Fprintln(os.Stdout, args...)
}

// Printf 将日志内容输出到控制台。
func (c *Console) Printf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, format, args...)
}

// Output 自定义日志级别和调用栈深度，skip 是相对于 Output 的调用栈深度。
func (c *Console) Output(skip int, level Level, args ...interface{}) {
	if c.level <= level {
		c.output(skip+1, level, fmt.Sprint(args...))
	}
}

// Outputf 自定义日志级别和调用栈深度，skip 是相对于 Output 的调用栈深度。
func (c *Console) Outputf(skip int, level Level, format string, args ...interface{}) {
	if c.level <= level {
		c.output(skip+1, level, fmt.Sprintf(format, args...))
	}
}

// defaultLogger 默认的日志输出器，输出等级为 INFO 级别。
var defaultLogger StdLogger = NewConsole(InfoLevel)

// SetLogger 设置新的日志输出器。
func SetLogger(logger StdLogger) {
	defaultLogger = logger
}

// Trace 输出 TRACE 级别的日志。
func Trace(args ...interface{}) {
	defaultLogger.Output(1, TraceLevel, args...)
}

// Tracef 输出 TRACE 级别的日志。
func Tracef(format string, args ...interface{}) {
	defaultLogger.Outputf(1, TraceLevel, format, args...)
}

// Debug 输出 DEBUG 级别的日志。
func Debug(args ...interface{}) {
	defaultLogger.Output(1, DebugLevel, args...)
}

// Debugf 输出 DEBUG 级别的日志。
func Debugf(format string, args ...interface{}) {
	defaultLogger.Outputf(1, DebugLevel, format, args...)
}

// Info 输出 INFO 级别的日志。
func Info(args ...interface{}) {
	defaultLogger.Output(1, InfoLevel, args...)
}

// Infof 输出 INFO 级别的日志。
func Infof(format string, args ...interface{}) {
	defaultLogger.Outputf(1, InfoLevel, format, args...)
}

// Warn 输出 WARN 级别的日志。
func Warn(args ...interface{}) {
	defaultLogger.Output(1, WarnLevel, args...)
}

// Warnf 输出 WARN 级别的日志。
func Warnf(format string, args ...interface{}) {
	defaultLogger.Outputf(1, WarnLevel, format, args...)
}

// Error 输出 ERROR 级别的日志。
func Error(args ...interface{}) {
	defaultLogger.Output(1, ErrorLevel, args...)
}

// Errorf 输出 ERROR 级别的日志。
func Errorf(format string, args ...interface{}) {
	defaultLogger.Outputf(1, ErrorLevel, format, args...)
}

// Panic 输出 PANIC 级别的日志。
func Panic(args ...interface{}) {
	defaultLogger.Output(1, PanicLevel, args...)
}

// Panicf 输出 PANIC 级别的日志。
func Panicf(format string, args ...interface{}) {
	defaultLogger.Outputf(1, PanicLevel, format, args...)
}

// Fatal 输出 FATAL 级别的日志。
func Fatal(args ...interface{}) {
	defaultLogger.Output(1, FatalLevel, args...)
}

// Fatalf 输出 FATAL 级别的日志。
func Fatalf(format string, args ...interface{}) {
	defaultLogger.Outputf(1, FatalLevel, format, args...)
}
