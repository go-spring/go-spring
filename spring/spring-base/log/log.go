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

// Package log 重新定义标准日志接口，可以灵活适配各种日志框架。
package log

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/go-spring/spring-base/color"
)

const (
	TraceLevel = Level(iota)
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	PanicLevel
	FatalLevel
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

// Entry 打包日志信息。
type Entry struct {
	ctx  context.Context
	tag  string
	msg  string
	file string
	line int
}

func (e *Entry) GetCtx() context.Context {
	return e.ctx
}

func (e *Entry) GetTag() string {
	return e.tag
}

func (e *Entry) GetMsg() string {
	return e.msg
}

func (e *Entry) GetFile() string {
	return e.file
}

func (e *Entry) GetLine() int {
	return e.line
}

func (e Entry) Tag(tag string) Entry {
	e.tag = tag
	return e
}

func (e Entry) Ctx(ctx context.Context) Entry {
	e.ctx = ctx
	return e
}

func (e Entry) format(format string, a ...interface{}) *Entry {
	if format == "" {
		e.msg = fmt.Sprint(a...)
	} else {
		e.msg = fmt.Sprintf(format, a...)
	}
	return &e
}

// Output 自定义日志的输出格式。
type Output func(level Level, e *Entry)

// Console 将日志输出到控制台。
func Console(level Level, e *Entry) {
	strLevel := strings.ToUpper(level.String())
	if level >= ErrorLevel {
		strLevel = color.Red.Sprint(strLevel)
	} else if level == WarnLevel {
		strLevel = color.Yellow.Sprint(strLevel)
	} else if level == TraceLevel {
		strLevel = color.Green.Sprint(strLevel)
	}
	_, _ = fmt.Printf("[%s] %s:%d %s\n", strLevel, e.file, e.line, e.msg)
}

var config = struct {
	mutex  sync.RWMutex
	level  Level
	output Output
}{
	level:  InfoLevel,
	output: Console,
}

// Reset 恢复默认的日志输出配置。
func Reset() {
	config.mutex.Lock()
	defer config.mutex.Unlock()
	config.level = InfoLevel
	config.output = Console
}

// GetLevel 获取日志的输出级别。
func GetLevel() Level {
	config.mutex.RLock()
	defer config.mutex.RUnlock()
	return config.level
}

// SetLevel 设置日志的输出级别。
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

var (
	frameMap sync.Map
)

// Caller 获取调用栈的文件及行号信息，fast 为 true 时使用缓存进行加速。
// 基准测试表明获取调用栈的信息时使用缓存相比不使用缓存有 50% 的速度提升。
func Caller(skip int, fast bool) (file string, line int, loaded bool) {

	if !fast {
		_, file, line, loaded = runtime.Caller(skip + 1)
		return
	}

	rpc := make([]uintptr, 1)
	n := runtime.Callers(skip+2, rpc[:])
	if n < 1 {
		return
	}
	pc := rpc[0]
	if v, ok := frameMap.Load(pc); ok {
		e := v.(*runtime.Frame)
		return e.File, e.Line, true
	}
	frame, _ := runtime.CallersFrames(rpc).Next()
	frameMap.Store(pc, &frame)
	return frame.File, frame.Line, false
}

func outputf(level Level, e Entry, format string, args ...interface{}) {

	var (
		configLevel  Level
		configOutput Output
	)

	{
		config.mutex.RLock()
		defer config.mutex.RUnlock()
		configLevel = config.level
		configOutput = config.output
	}

	if configLevel > level {
		return
	}
	if len(args) == 1 {
		if fn, ok := args[0].(func() []interface{}); ok {
			args = fn()
		}
	}
	e.file, e.line, _ = Caller(2, true)
	configOutput(level, e.format(format, args...))
}

// T 将可变参数转换成切片形式。
func T(a ...interface{}) []interface{} {
	return a
}

// Trace 输出 TRACE 级别的日志。
func (e Entry) Trace(args ...interface{}) {
	outputf(TraceLevel, e, "", args...)
}

// Tracef 输出 TRACE 级别的日志。
func (e Entry) Tracef(format string, args ...interface{}) {
	outputf(TraceLevel, e, format, args...)
}

// Debug 输出 DEBUG 级别的日志。
func (e Entry) Debug(args ...interface{}) {
	outputf(DebugLevel, e, "", args...)
}

// Debugf 输出 DEBUG 级别的日志。
func (e Entry) Debugf(format string, args ...interface{}) {
	outputf(DebugLevel, e, format, args...)
}

// Info 输出 INFO 级别的日志。
func (e Entry) Info(args ...interface{}) {
	outputf(InfoLevel, e, "", args...)
}

// Infof 输出 INFO 级别的日志。
func (e Entry) Infof(format string, args ...interface{}) {
	outputf(InfoLevel, e, format, args...)
}

// Warn 输出 WARN 级别的日志。
func (e Entry) Warn(args ...interface{}) {
	outputf(WarnLevel, e, "", args...)
}

// Warnf 输出 WARN 级别的日志。
func (e Entry) Warnf(format string, args ...interface{}) {
	outputf(WarnLevel, e, format, args...)
}

// Error 输出 ERROR 级别的日志。
func (e Entry) Error(args ...interface{}) {
	outputf(ErrorLevel, e, "", args...)
}

// Errorf 输出 ERROR 级别的日志。
func (e Entry) Errorf(format string, args ...interface{}) {
	outputf(ErrorLevel, e, format, args...)
}

// Panic 输出 PANIC 级别的日志。
func (e Entry) Panic(args ...interface{}) {
	outputf(PanicLevel, e, "", args...)
}

// Panicf 输出 PANIC 级别的日志。
func (e Entry) Panicf(format string, args ...interface{}) {
	outputf(PanicLevel, e, format, args...)
}

// Fatal 输出 FATAL 级别的日志。
func (e Entry) Fatal(args ...interface{}) {
	outputf(FatalLevel, e, "", args...)
}

// Fatalf 输出 FATAL 级别的日志。
func (e Entry) Fatalf(format string, args ...interface{}) {
	outputf(FatalLevel, e, format, args...)
}

// EnableTrace 是否允许输出 TRACE 级别的日志。
func EnableTrace() bool {
	return GetLevel() <= TraceLevel
}

// EnableDebug 是否允许输出 DEBUG 级别的日志。
func EnableDebug() bool {
	return GetLevel() <= DebugLevel
}

// EnableInfo 是否允许输出 INFO 级别的日志。
func EnableInfo() bool {
	return GetLevel() <= InfoLevel
}

// EnableWarn 是否允许输出 WARN 级别的日志。
func EnableWarn() bool {
	return GetLevel() <= WarnLevel
}

// EnableError 是否允许输出 ERROR 级别的日志。
func EnableError() bool {
	return GetLevel() <= ErrorLevel
}

// EnablePanic 是否允许输出 PANIC 级别的日志。
func EnablePanic() bool {
	return GetLevel() <= PanicLevel
}

// EnableFatal 是否允许输出 FATAL 级别的日志。
func EnableFatal() bool {
	return GetLevel() <= FatalLevel
}

// Trace 输出 TRACE 级别的日志。
func Trace(args ...interface{}) {
	outputf(TraceLevel, empty, "", args...)
}

// Tracef 输出 TRACE 级别的日志。
func Tracef(format string, args ...interface{}) {
	outputf(TraceLevel, empty, format, args...)
}

// Debug 输出 DEBUG 级别的日志。
func Debug(args ...interface{}) {
	outputf(DebugLevel, empty, "", args...)
}

// Debugf 输出 DEBUG 级别的日志。
func Debugf(format string, args ...interface{}) {
	outputf(DebugLevel, empty, format, args...)
}

// Info 输出 INFO 级别的日志。
func Info(args ...interface{}) {
	outputf(InfoLevel, empty, "", args...)
}

// Infof 输出 INFO 级别的日志。
func Infof(format string, args ...interface{}) {
	outputf(InfoLevel, empty, format, args...)
}

// Warn 输出 WARN 级别的日志。
func Warn(args ...interface{}) {
	outputf(WarnLevel, empty, "", args...)
}

// Warnf 输出 WARN 级别的日志。
func Warnf(format string, args ...interface{}) {
	outputf(WarnLevel, empty, format, args...)
}

// Error 输出 ERROR 级别的日志。
func Error(args ...interface{}) {
	outputf(ErrorLevel, empty, "", args...)
}

// Errorf 输出 ERROR 级别的日志。
func Errorf(format string, args ...interface{}) {
	outputf(ErrorLevel, empty, format, args...)
}

// Panic 输出 PANIC 级别的日志。
func Panic(args ...interface{}) {
	outputf(PanicLevel, empty, "", args...)
}

// Panicf 输出 PANIC 级别的日志。
func Panicf(format string, args ...interface{}) {
	outputf(PanicLevel, empty, format, args...)
}

// Fatal 输出 FATAL 级别的日志。
func Fatal(args ...interface{}) {
	outputf(FatalLevel, empty, "", args...)
}

// Fatalf 输出 FATAL 级别的日志。
func Fatalf(format string, args ...interface{}) {
	outputf(FatalLevel, empty, format, args...)
}
