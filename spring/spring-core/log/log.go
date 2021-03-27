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

package log

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
)

const (
	TraceLevel = Level(0)
	DebugLevel = Level(1)
	InfoLevel  = Level(2)
	WarnLevel  = Level(3)
	ErrorLevel = Level(4)
	PanicLevel = Level(5)
	FatalLevel = Level(6)
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

var entry = Entry{}

// Ctx
func Ctx(ctx context.Context) Entry {
	return entry.Ctx(ctx)
}

// Tag
func Tag(tag string) Entry {
	return entry.Tag(tag)
}

// Entry
type Entry struct {
	ctx context.Context
	tag string
	msg string
}

func (e *Entry) GetMsg() string {
	return e.msg
}

func (e *Entry) GetTag() string {
	return e.tag
}

func (e *Entry) GetCtx() context.Context {
	return e.ctx
}

func (e Entry) Tag(tag string) Entry {
	e.tag = tag
	return e
}

func (e Entry) Ctx(ctx context.Context) Entry {
	e.ctx = ctx
	return e
}

func (e Entry) print(a ...interface{}) *Entry {
	e.msg = fmt.Sprint(a...)
	return &e
}

func (e Entry) printf(format string, a ...interface{}) *Entry {
	e.msg = fmt.Sprintf(format, a...)
	return &e
}

// Output 定制日志的输出格式，skip 是相对于当前函数的调用深度。
type Output func(skip int, level Level, e *Entry)

func DefaultOutput(skip int, level Level, e *Entry) {

	strLevel := strings.ToUpper(level.String())
	if level >= ErrorLevel {
		strLevel = fmt.Sprintf("\x1b[31m%s\x1b[0m", strLevel) // RED
	} else if level == WarnLevel {
		strLevel = fmt.Sprintf("\x1b[33m%s\x1b[0m", strLevel) // YELLOW
	}

	_, file, line, _ := runtime.Caller(skip + 1)
	fmt.Fprintf(os.Stdout, "[%s] %s:%d %s\n", strLevel, file, line, e.GetMsg())

	switch level {
	case PanicLevel:
		panic(e.GetMsg())
	case FatalLevel:
		os.Exit(1)
	}
}

var config = struct {
	mutex  sync.Mutex
	level  Level
	output Output
}{
	level:  InfoLevel,
	output: DefaultOutput,
}

// SetLevel 设置日志输出的级别
func SetLevel(level Level) Level {

	config.mutex.Lock()
	defer config.mutex.Unlock()

	old := config.level
	config.level = level
	return old
}

// SetOutput 设置日志的输出格式
func SetOutput(output Output) Output {

	config.mutex.Lock()
	defer config.mutex.Unlock()

	old := config.output
	config.output = output
	return old
}

// Trace 输出 TRACE 级别的日志。
func (e Entry) Trace(args ...interface{}) {
	if config.level <= TraceLevel {
		config.output(1, TraceLevel, e.print(args...))
	}
}

// Tracef 输出 TRACE 级别的日志。
func (e Entry) Tracef(format string, args ...interface{}) {
	if config.level <= TraceLevel {
		config.output(1, TraceLevel, e.printf(format, args...))
	}
}

// Debug 输出 DEBUG 级别的日志。
func (e Entry) Debug(args ...interface{}) {
	if config.level <= DebugLevel {
		config.output(1, DebugLevel, e.print(args...))
	}
}

// Debugf 输出 DEBUG 级别的日志。
func (e Entry) Debugf(format string, args ...interface{}) {
	if config.level <= DebugLevel {
		config.output(1, DebugLevel, e.printf(format, args...))
	}
}

// Info 输出 INFO 级别的日志。
func (e Entry) Info(args ...interface{}) {
	if config.level <= InfoLevel {
		config.output(1, InfoLevel, e.print(args...))
	}
}

// Infof 输出 INFO 级别的日志。
func (e Entry) Infof(format string, args ...interface{}) {
	if config.level <= InfoLevel {
		config.output(1, InfoLevel, e.printf(format, args...))
	}
}

// Warn 输出 WARN 级别的日志。
func (e Entry) Warn(args ...interface{}) {
	if config.level <= WarnLevel {
		config.output(1, WarnLevel, e.print(args...))
	}
}

// Warnf 输出 WARN 级别的日志。
func (e Entry) Warnf(format string, args ...interface{}) {
	if config.level <= WarnLevel {
		config.output(1, WarnLevel, e.printf(format, args...))
	}
}

// Error 输出 ERROR 级别的日志。
func (e Entry) Error(args ...interface{}) {
	if config.level <= ErrorLevel {
		config.output(1, ErrorLevel, e.print(args...))
	}
}

// Errorf 输出 ERROR 级别的日志。
func (e Entry) Errorf(format string, args ...interface{}) {
	if config.level <= ErrorLevel {
		config.output(1, ErrorLevel, e.printf(format, args...))
	}
}

// Panic 输出 PANIC 级别的日志。
func (e Entry) Panic(args ...interface{}) {
	if config.level <= PanicLevel {
		config.output(1, PanicLevel, e.print(args...))
	}
}

// Panicf 输出 PANIC 级别的日志。
func (e Entry) Panicf(format string, args ...interface{}) {
	if config.level <= PanicLevel {
		config.output(1, PanicLevel, e.printf(format, args...))
	}
}

// Fatal 输出 FATAL 级别的日志。
func (e Entry) Fatal(args ...interface{}) {
	if config.level <= FatalLevel {
		config.output(1, FatalLevel, e.print(args...))
	}
}

// Fatalf 输出 FATAL 级别的日志。
func (e Entry) Fatalf(format string, args ...interface{}) {
	if config.level <= FatalLevel {
		config.output(1, FatalLevel, e.printf(format, args...))
	}
}

// Trace 输出 TRACE 级别的日志。
func Trace(args ...interface{}) {
	if config.level <= TraceLevel {
		config.output(1, TraceLevel, entry.print(args...))
	}
}

// Tracef 输出 TRACE 级别的日志。
func Tracef(format string, args ...interface{}) {
	if config.level <= TraceLevel {
		config.output(1, TraceLevel, entry.printf(format, args...))
	}
}

// Debug 输出 DEBUG 级别的日志。
func Debug(args ...interface{}) {
	if config.level <= DebugLevel {
		config.output(1, DebugLevel, entry.print(args...))
	}
}

// Debugf 输出 DEBUG 级别的日志。
func Debugf(format string, args ...interface{}) {
	if config.level <= DebugLevel {
		config.output(1, DebugLevel, entry.printf(format, args...))
	}
}

// Info 输出 INFO 级别的日志。
func Info(args ...interface{}) {
	if config.level <= InfoLevel {
		config.output(1, InfoLevel, entry.print(args...))
	}
}

// Infof 输出 INFO 级别的日志。
func Infof(format string, args ...interface{}) {
	if config.level <= InfoLevel {
		config.output(1, InfoLevel, entry.printf(format, args...))
	}
}

// Warn 输出 WARN 级别的日志。
func Warn(args ...interface{}) {
	if config.level <= WarnLevel {
		config.output(1, WarnLevel, entry.print(args...))
	}
}

// Warnf 输出 WARN 级别的日志。
func Warnf(format string, args ...interface{}) {
	if config.level <= WarnLevel {
		config.output(1, WarnLevel, entry.printf(format, args...))
	}
}

// Error 输出 ERROR 级别的日志。
func Error(args ...interface{}) {
	if config.level <= ErrorLevel {
		config.output(1, ErrorLevel, entry.print(args...))
	}
}

// Errorf 输出 ERROR 级别的日志。
func Errorf(format string, args ...interface{}) {
	if config.level <= ErrorLevel {
		config.output(1, ErrorLevel, entry.printf(format, args...))
	}
}

// Panic 输出 PANIC 级别的日志。
func Panic(args ...interface{}) {
	if config.level <= PanicLevel {
		config.output(1, PanicLevel, entry.print(args...))
	}
}

// Panicf 输出 PANIC 级别的日志。
func Panicf(format string, args ...interface{}) {
	if config.level <= PanicLevel {
		config.output(1, PanicLevel, entry.printf(format, args...))
	}
}

// Fatal 输出 FATAL 级别的日志。
func Fatal(args ...interface{}) {
	if config.level <= FatalLevel {
		config.output(1, FatalLevel, entry.print(args...))
	}
}

// Fatalf 输出 FATAL 级别的日志。
func Fatalf(format string, args ...interface{}) {
	if config.level <= FatalLevel {
		config.output(1, FatalLevel, entry.printf(format, args...))
	}
}
