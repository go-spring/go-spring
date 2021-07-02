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

// Package log 实现了重新定义的标准日志接口，可以灵活适配各种日志框架。
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

func (level Level) String() string {
	switch level {
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

var empty = Entry{}

// Ctx 创建包含 context.Context 对象的 Entry 。
func Ctx(ctx context.Context) Entry {
	return empty.Ctx(ctx)
}

// Tag 创建包含 tag 信息的 Entry 。
func Tag(tag string) Entry {
	return empty.Tag(tag)
}

// Entry 打包需要记录的日志信息。
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

// Console 将日志输出到控制台。
func Console(skip int, level Level, e *Entry) {

	strLevel := strings.ToUpper(level.String())
	if level >= ErrorLevel {
		strLevel = fmt.Sprintf("\x1b[31m%s\x1b[0m", strLevel) // RED
	} else if level == WarnLevel {
		strLevel = fmt.Sprintf("\x1b[33m%s\x1b[0m", strLevel) // YELLOW
	}

	_, file, line, _ := runtime.Caller(skip + 1)
	_, _ = fmt.Printf("[%s] %s:%d %s\n", strLevel, file, line, e.GetMsg())

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
	output: Console,
}

// T 将可变参数转换成切片。
func T(a ...interface{}) []interface{} { return a }

func output(level Level, e Entry, args ...interface{}) {
	if config.level <= level {
		if len(args) == 1 {
			if fn, ok := args[0].(func() []interface{}); ok {
				args = fn()
			}
		}
		config.output(2, level, e.print(args...))
	}
}

func outputf(level Level, e Entry, format string, args ...interface{}) {
	if config.level <= level {
		if len(args) == 1 {
			if fn, ok := args[0].(func() []interface{}); ok {
				args = fn()
			}
		}
		config.output(2, level, e.printf(format, args...))
	}
}

// Reset 重新设置输出级别及输出格式。
func Reset() {
	config.mutex.Lock()
	defer config.mutex.Unlock()
	config.level = InfoLevel
	config.output = Console
}

// SetLevel 设置日志输出的级别。
func SetLevel(level Level) {
	config.mutex.Lock()
	defer config.mutex.Unlock()
	config.level = level
}

// SetOutput 设置日志的输出格式。
func SetOutput(output Output) {
	config.mutex.Lock()
	defer config.mutex.Unlock()
	config.output = output
}

// Trace 输出 TRACE 级别的日志。
func (e Entry) Trace(args ...interface{}) {
	output(TraceLevel, e, args...)
}

// Tracef 输出 TRACE 级别的日志。
func (e Entry) Tracef(format string, args ...interface{}) {
	outputf(TraceLevel, e, format, args...)
}

// Debug 输出 DEBUG 级别的日志。
func (e Entry) Debug(args ...interface{}) {
	output(DebugLevel, e, args...)
}

// Debugf 输出 DEBUG 级别的日志。
func (e Entry) Debugf(format string, args ...interface{}) {
	outputf(DebugLevel, e, format, args...)
}

// Info 输出 INFO 级别的日志。
func (e Entry) Info(args ...interface{}) {
	output(InfoLevel, e, args...)
}

// Infof 输出 INFO 级别的日志。
func (e Entry) Infof(format string, args ...interface{}) {
	outputf(InfoLevel, e, format, args...)
}

// Warn 输出 WARN 级别的日志。
func (e Entry) Warn(args ...interface{}) {
	output(WarnLevel, e, args...)
}

// Warnf 输出 WARN 级别的日志。
func (e Entry) Warnf(format string, args ...interface{}) {
	outputf(WarnLevel, e, format, args...)
}

// Error 输出 ERROR 级别的日志。
func (e Entry) Error(args ...interface{}) {
	output(ErrorLevel, e, args...)
}

// Errorf 输出 ERROR 级别的日志。
func (e Entry) Errorf(format string, args ...interface{}) {
	outputf(ErrorLevel, e, format, args...)
}

// Panic 输出 PANIC 级别的日志。
func (e Entry) Panic(args ...interface{}) {
	output(PanicLevel, e, args...)
}

// Panicf 输出 PANIC 级别的日志。
func (e Entry) Panicf(format string, args ...interface{}) {
	outputf(PanicLevel, e, format, args...)
}

// Fatal 输出 FATAL 级别的日志。
func (e Entry) Fatal(args ...interface{}) {
	output(FatalLevel, e, args...)
}

// Fatalf 输出 FATAL 级别的日志。
func (e Entry) Fatalf(format string, args ...interface{}) {
	outputf(FatalLevel, e, format, args...)
}

// EnableTrace 是否允许输出 TRACE 级别的日志。
func EnableTrace() bool {
	return config.level <= TraceLevel
}

// EnableDebug 是否允许输出 DEBUG 级别的日志。
func EnableDebug() bool {
	return config.level <= DebugLevel
}

// EnableInfo 是否允许输出 INFO 级别的日志。
func EnableInfo() bool {
	return config.level <= InfoLevel
}

// EnableWarn 是否允许输出 WARN 级别的日志。
func EnableWarn() bool {
	return config.level <= WarnLevel
}

// EnableError 是否允许输出 ERROR 级别的日志。
func EnableError() bool {
	return config.level <= ErrorLevel
}

// EnablePanic 是否允许输出 PANIC 级别的日志。
func EnablePanic() bool {
	return config.level <= PanicLevel
}

// EnableFatal 是否允许输出 FATAL 级别的日志。
func EnableFatal() bool {
	return config.level <= FatalLevel
}

// Trace 输出 TRACE 级别的日志。
func Trace(args ...interface{}) {
	output(TraceLevel, empty, args...)
}

// Tracef 输出 TRACE 级别的日志。
func Tracef(format string, args ...interface{}) {
	outputf(TraceLevel, empty, format, args...)
}

// Debug 输出 DEBUG 级别的日志。
func Debug(args ...interface{}) {
	output(DebugLevel, empty, args...)
}

// Debugf 输出 DEBUG 级别的日志。
func Debugf(format string, args ...interface{}) {
	outputf(DebugLevel, empty, format, args...)
}

// Info 输出 INFO 级别的日志。
func Info(args ...interface{}) {
	output(InfoLevel, empty, args...)
}

// Infof 输出 INFO 级别的日志。
func Infof(format string, args ...interface{}) {
	outputf(InfoLevel, empty, format, args...)
}

// Warn 输出 WARN 级别的日志。
func Warn(args ...interface{}) {
	output(WarnLevel, empty, args...)
}

// Warnf 输出 WARN 级别的日志。
func Warnf(format string, args ...interface{}) {
	outputf(WarnLevel, empty, format, args...)
}

// Error 输出 ERROR 级别的日志。
func Error(args ...interface{}) {
	output(ErrorLevel, empty, args...)
}

// Errorf 输出 ERROR 级别的日志。
func Errorf(format string, args ...interface{}) {
	outputf(ErrorLevel, empty, format, args...)
}

// Panic 输出 PANIC 级别的日志。
func Panic(args ...interface{}) {
	output(PanicLevel, empty, args...)
}

// Panicf 输出 PANIC 级别的日志。
func Panicf(format string, args ...interface{}) {
	outputf(PanicLevel, empty, format, args...)
}

// Fatal 输出 FATAL 级别的日志。
func Fatal(args ...interface{}) {
	output(FatalLevel, empty, args...)
}

// Fatalf 输出 FATAL 级别的日志。
func Fatalf(format string, args ...interface{}) {
	outputf(FatalLevel, empty, format, args...)
}
