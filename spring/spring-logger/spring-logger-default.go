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

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/labstack/gommon/color"
)

// defaultLogger 默认的日志输出器，输出等级为 INFO 级别。
var defaultLogger StdLogger = NewConsole(InfoLevel)

// SetLogger 设置新的日志输出器
func SetLogger(logger StdLogger) {
	defaultLogger = logger
}

// SetLevel 设置日志的输出级别，请确保线程安全
func SetLevel(level Level) {
	defaultLogger.SetLevel(level)
}

// Trace 输出 TRACE 级别的日志
func Trace(args ...interface{}) {
	defaultLogger.Output(1, TraceLevel, args...)
}

// Tracef 输出 TRACE 级别的日志
func Tracef(format string, args ...interface{}) {
	defaultLogger.Outputf(1, TraceLevel, format, args...)
}

// Debug 输出 DEBUG 级别的日志
func Debug(args ...interface{}) {
	defaultLogger.Output(1, DebugLevel, args...)
}

// Debugf 输出 DEBUG 级别的日志
func Debugf(format string, args ...interface{}) {
	defaultLogger.Outputf(1, DebugLevel, format, args...)
}

// Info 输出 INFO 级别的日志
func Info(args ...interface{}) {
	defaultLogger.Output(1, InfoLevel, args...)
}

// Infof 输出 INFO 级别的日志
func Infof(format string, args ...interface{}) {
	defaultLogger.Outputf(1, InfoLevel, format, args...)
}

// Warn 输出 WARN 级别的日志
func Warn(args ...interface{}) {
	defaultLogger.Output(1, WarnLevel, args...)
}

// Warnf 输出 WARN 级别的日志
func Warnf(format string, args ...interface{}) {
	defaultLogger.Outputf(1, WarnLevel, format, args...)
}

// Error 输出 ERROR 级别的日志
func Error(args ...interface{}) {
	defaultLogger.Output(1, ErrorLevel, args...)
}

// Errorf 输出 ERROR 级别的日志
func Errorf(format string, args ...interface{}) {
	defaultLogger.Outputf(1, ErrorLevel, format, args...)
}

// Panic 输出 PANIC 级别的日志
func Panic(args ...interface{}) {
	defaultLogger.Output(1, PanicLevel, args...)
}

// Panicf 输出 PANIC 级别的日志
func Panicf(format string, args ...interface{}) {
	defaultLogger.Outputf(1, PanicLevel, format, args...)
}

// Fatal 输出 FATAL 级别的日志
func Fatal(args ...interface{}) {
	defaultLogger.Output(1, FatalLevel, args...)
}

// Fatalf 输出 FATAL 级别的日志
func Fatalf(format string, args ...interface{}) {
	defaultLogger.Outputf(1, FatalLevel, format, args...)
}

// Output 自定义日志级别和调用栈深度，skip 是相对于 Output 的调用深度
func Output(skip int, level Level, args ...interface{}) {
	defaultLogger.Output(skip+1, level, args...)
}

// Outputf 自定义日志级别和调用栈深度，skip 是相对于 Outputf 的调用深度
func Outputf(skip int, level Level, format string, args ...interface{}) {
	defaultLogger.Outputf(skip+1, level, format, args...)
}

// Console 将日志输出到控制台，StdLogger 的默认实现
type Console struct {
	level Level
}

// NewConsole Console 的构造函数
func NewConsole(level Level) *Console {
	return &Console{level: level}
}

// SetLevel 设置日志的输出级别，请确保线程安全
func (c *Console) SetLevel(level Level) {
	atomic.StoreUint32((*uint32)(&c.level), uint32(level))
}

// Trace 输出 TRACE 级别的日志
func (c *Console) Trace(args ...interface{}) {
	if c.level <= TraceLevel {
		c.Output(1, TraceLevel, args...)
	}
}

// Tracef 输出 TRACE 级别的日志
func (c *Console) Tracef(format string, args ...interface{}) {
	if c.level <= TraceLevel {
		c.Outputf(1, TraceLevel, format, args...)
	}
}

// Debug 输出 DEBUG 级别的日志
func (c *Console) Debug(args ...interface{}) {
	if c.level <= DebugLevel {
		c.Output(1, DebugLevel, args...)
	}
}

// Debugf 输出 DEBUG 级别的日志
func (c *Console) Debugf(format string, args ...interface{}) {
	if c.level <= DebugLevel {
		c.Outputf(1, DebugLevel, format, args...)
	}
}

// Info 输出 INFO 级别的日志
func (c *Console) Info(args ...interface{}) {
	if c.level <= InfoLevel {
		c.Output(1, InfoLevel, args...)
	}
}

// Infof 输出 INFO 级别的日志
func (c *Console) Infof(format string, args ...interface{}) {
	if c.level <= InfoLevel {
		c.Outputf(1, InfoLevel, format, args...)
	}
}

// Warn 输出 WARN 级别的日志
func (c *Console) Warn(args ...interface{}) {
	if c.level <= WarnLevel {
		c.Output(1, WarnLevel, args...)
	}
}

// Warnf 输出 WARN 级别的日志
func (c *Console) Warnf(format string, args ...interface{}) {
	if c.level <= WarnLevel {
		c.Outputf(1, WarnLevel, format, args...)
	}
}

// Error 输出 ERROR 级别的日志
func (c *Console) Error(args ...interface{}) {
	if c.level <= ErrorLevel {
		c.Output(1, ErrorLevel, args...)
	}
}

// Errorf 输出 ERROR 级别的日志
func (c *Console) Errorf(format string, args ...interface{}) {
	if c.level <= ErrorLevel {
		c.Outputf(1, ErrorLevel, format, args...)
	}
}

// Panic 输出 PANIC 级别的日志
func (c *Console) Panic(args ...interface{}) {
	if c.level <= PanicLevel {
		c.Output(1, PanicLevel, args...)
	}
}

// Panicf 输出 PANIC 级别的日志
func (c *Console) Panicf(format string, args ...interface{}) {
	if c.level <= PanicLevel {
		c.Outputf(1, PanicLevel, format, args...)
	}
}

// Fatal 输出 FATAL 级别的日志
func (c *Console) Fatal(args ...interface{}) {
	if c.level <= FatalLevel {
		c.Output(1, FatalLevel, args...)
	}
}

// Fatalf 输出 FATAL 级别的日志
func (c *Console) Fatalf(format string, args ...interface{}) {
	if c.level <= FatalLevel {
		c.Outputf(1, FatalLevel, format, args...)
	}
}

// Print 将日志内容输出到控制台
func (c *Console) Print(args ...interface{}) {
	fmt.Fprintln(os.Stdout, args...)
}

// Printf 将日志内容输出到控制台
func (c *Console) Printf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, format, args...)
}

// Output 自定义日志级别和调用栈深度，skip 是相对于 Output 的调用栈深度
func (c *Console) Output(skip int, level Level, args ...interface{}) {
	if c.level <= level {
		c.log(skip+1, level, fmt.Sprint(args...))
	}
}

// Outputf 自定义日志级别和调用栈深度，skip 是相对于 Output 的调用栈深度
func (c *Console) Outputf(skip int, level Level, format string, args ...interface{}) {
	if c.level <= level {
		c.log(skip+1, level, fmt.Sprintf(format, args...))
	}
}

func (c *Console) log(skip int, level Level, msg string) {
	strLevel := strings.ToUpper(level.String())

	if level >= ErrorLevel {
		strLevel = color.Red(strLevel)
	} else if level == WarnLevel {
		strLevel = color.Yellow(strLevel)
	}

	_, file, line, _ := runtime.Caller(skip + 1)
	dir, filename := path.Split(file)
	filename = path.Join(path.Base(dir), filename)

	// TODO 可以让用户自定义日志的输出格式
	fmt.Fprintf(os.Stdout, "[%s] %s:%d %s\n", strLevel, filename, line, msg)

	switch level {
	case PanicLevel:
		panic(msg)
	case FatalLevel:
		os.Exit(1)
	}
}
