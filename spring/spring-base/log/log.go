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

//go:generate mockgen -source=log.go -destination=mock.go -package=log

// Package log 重新定义标准日志接口，可以灵活适配各种日志框架。
package log

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/chrono"
	"github.com/go-spring/spring-base/color"
	"github.com/go-spring/spring-base/util"
)

const (
	NoneLevel  = Level(-1)
	TraceLevel = Level(iota)
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	PanicLevel
	FatalLevel
)

var (
	Console        = newConsole()
	outputs        = make(map[string]Output)
	emptyEntry     = &BaseEntry{}
	defaultContext context.Context
)

// Output 自定义日志的输出格式。
type Output interface {
	Level() Level
	Print(msg *Message)
}

// Level 日志输出级别。
type Level int32

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
	default:
		return ""
	}
}

func ClearOutputs() {
	outputs = make(map[string]Output)
}

func GetOutput(tag string) Output {
	v, ok := outputs[tag]
	if !ok {
		return Console
	}
	return v
}

func RegisterDefaultOutput(outputf Output) {
	_, ok := outputs[""]
	if ok {
		panic(errors.New("duplicate default outputf"))
	}
	outputs[""] = outputf
}

func RegisterOutput(outputf Output, tags ...string) {
	for _, tag := range tags {
		_, ok := outputs[tag]
		if ok {
			panic(errors.New("duplicate outputf for tag " + tag))
		}
		outputs[tag] = outputf
	}
}

type console struct {
	level atomic.Int32
}

func newConsole() *console {
	c := &console{}
	c.SetLevel(InfoLevel)
	return c
}

func (c *console) Level() Level {
	return Level(c.level.Load())
}

func (c *console) SetLevel(level Level) {
	c.level.Store(int32(level))
}

func (c *console) Print(msg *Message) {
	defer func() { msg.Reuse() }()
	level := msg.Level()
	strLevel := strings.ToUpper(level.String())
	if level >= ErrorLevel {
		strLevel = color.Red.Sprint(strLevel)
	} else if level == WarnLevel {
		strLevel = color.Yellow.Sprint(strLevel)
	} else if level == TraceLevel {
		strLevel = color.Green.Sprint(strLevel)
	}
	var buf bytes.Buffer
	for _, a := range msg.Args() {
		buf.WriteString(cast.ToString(a))
	}
	strTime := msg.Time().Format("2006-01-02T15:04:05.000")
	fileLine := util.Contract(fmt.Sprintf("%s:%d", msg.File(), msg.Line()), 48)
	_, _ = fmt.Printf("[%s][%s][%s] %s\n", strLevel, strTime, fileLine, buf.String())
}

// WithSkip 创建包含 skip 信息的 Entry 。
func WithSkip(n int) BaseEntry {
	return emptyEntry.WithSkip(n)
}

// WithTag 创建包含 tag 信息的 Entry 。
func WithTag(tag string) BaseEntry {
	return emptyEntry.WithTag(tag)
}

// WithContext 创建包含 context.Context 对象的 Entry 。
func WithContext(ctx context.Context) CtxEntry {
	return emptyEntry.WithContext(ctx)
}

// T 将可变参数转换成切片形式。
func T(a ...interface{}) []interface{} {
	return a
}

// Trace 输出 TRACE 级别的日志。
func Trace(args ...interface{}) {
	outputf(TraceLevel, emptyEntry, "", args)
}

// Tracef 输出 TRACE 级别的日志。
func Tracef(format string, args ...interface{}) {
	outputf(TraceLevel, emptyEntry, format, args)
}

// Debug 输出 DEBUG 级别的日志。
func Debug(args ...interface{}) {
	outputf(DebugLevel, emptyEntry, "", args)
}

// Debugf 输出 DEBUG 级别的日志。
func Debugf(format string, args ...interface{}) {
	outputf(DebugLevel, emptyEntry, format, args)
}

// Info 输出 INFO 级别的日志。
func Info(args ...interface{}) {
	outputf(InfoLevel, emptyEntry, "", args)
}

// Infof 输出 INFO 级别的日志。
func Infof(format string, args ...interface{}) {
	outputf(InfoLevel, emptyEntry, format, args)
}

// Warn 输出 WARN 级别的日志。
func Warn(args ...interface{}) {
	outputf(WarnLevel, emptyEntry, "", args)
}

// Warnf 输出 WARN 级别的日志。
func Warnf(format string, args ...interface{}) {
	outputf(WarnLevel, emptyEntry, format, args)
}

// Error 输出 ERROR 级别的日志。
func Error(args ...interface{}) {
	outputf(ErrorLevel, emptyEntry, "", args)
}

// Errorf 输出 ERROR 级别的日志。
func Errorf(format string, args ...interface{}) {
	outputf(ErrorLevel, emptyEntry, format, args)
}

// Panic 输出 PANIC 级别的日志。
func Panic(args ...interface{}) {
	outputf(PanicLevel, emptyEntry, "", args)
}

// Panicf 输出 PANIC 级别的日志。
func Panicf(format string, args ...interface{}) {
	outputf(PanicLevel, emptyEntry, format, args)
}

// Fatal 输出 FATAL 级别的日志。
func Fatal(args ...interface{}) {
	outputf(FatalLevel, emptyEntry, "", args)
}

// Fatalf 输出 FATAL 级别的日志。
func Fatalf(format string, args ...interface{}) {
	outputf(FatalLevel, emptyEntry, format, args)
}

func outputf(level Level, e Entry, format string, args []interface{}) {
	o := GetOutput(e.Tag())
	if o.Level() > level {
		return
	}
	if len(args) == 1 {
		if fn, ok := args[0].(func() []interface{}); ok {
			args = fn()
		}
	}
	if format == "" {
		doPrint(o, level, e, args)
		return
	}
	doPrint(o, level, e, []interface{}{fmt.Sprintf(format, args...)})
}

func doPrint(o Output, level Level, e Entry, args []interface{}) {
	msg := newMessage()
	msg.level = level
	msg.args = args
	msg.tag = e.Tag()
	msg.ctx = e.Context()
	msg.errno = e.Errno()
	ctx := msg.ctx
	if ctx == nil {
		ctx = defaultContext
	}
	msg.time = chrono.Now(ctx)
	msg.file, msg.line, _ = Caller(e.Skip()+3, true)
	o.Print(msg)
}
