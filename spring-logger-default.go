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
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/labstack/gommon/color"
)

// defaultLogger 默认的日志输出器
var defaultLogger StdLogger = NewConsole(InfoLevel)

// SetLogger 设置新的日志输出器
func SetLogger(logger StdLogger) {
	defaultLogger = logger
}

// Trace 打印 TRACE 日志
func Trace(args ...interface{}) {
	defaultLogger.Trace(args...)
}

// Tracef 打印 TRACE 日志
func Tracef(format string, args ...interface{}) {
	defaultLogger.Tracef(format, args...)
}

// Debug 打印 DEBUG 日志
func Debug(args ...interface{}) {
	defaultLogger.Debug(args...)
}

// Debugf 打印 DEBUG 日志
func Debugf(format string, args ...interface{}) {
	defaultLogger.Debugf(format, args...)
}

// Info 打印 INFO 日志
func Info(args ...interface{}) {
	defaultLogger.Info(args...)
}

// Infof 打印 INFO 日志
func Infof(format string, args ...interface{}) {
	defaultLogger.Infof(format, args...)
}

// Warn 打印 WARN 日志
func Warn(args ...interface{}) {
	defaultLogger.Warn(args...)
}

// Warnf 打印 WARN 日志
func Warnf(format string, args ...interface{}) {
	defaultLogger.Warnf(format, args...)
}

// Error 打印 ERROR 日志
func Error(args ...interface{}) {
	defaultLogger.Error(args...)
}

// Errorf 打印 ERROR 日志
func Errorf(format string, args ...interface{}) {
	defaultLogger.Errorf(format, args...)
}

// Panic 打印 PANIC 日志
func Panic(args ...interface{}) {
	defaultLogger.Panic(args...)
}

// Panicf 打印 PANIC 日志
func Panicf(format string, args ...interface{}) {
	defaultLogger.Panicf(format, args...)
}

// Fatal 打印 FATAL 日志
func Fatal(args ...interface{}) {
	defaultLogger.Fatal(args...)
}

// Fatalf 打印 FATAL 日志
func Fatalf(format string, args ...interface{}) {
	defaultLogger.Fatalf(format, args...)
}

// Level 日志输出级别
type Level uint32

const (
	TraceLevel Level = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	PanicLevel
	FatalLevel
)

// LevelToString 返回 Level 对应的字符串
func LevelToString(l Level) string {
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
	panic(errors.New("error log level"))
}

// Console 将日志打印到控制台
type Console struct {
	level Level
}

// NewConsole Console 的构造函数
func NewConsole(level Level) *Console {
	return &Console{
		level: level,
	}
}

// SetLevel 设置日志的输出级别，线程安全
func (c *Console) SetLevel(level Level) {
	atomic.StoreUint32((*uint32)(&c.level), uint32(level))
}

// Trace 打印 TRACE 日志
func (c *Console) Trace(args ...interface{}) {
	if c.level <= TraceLevel {
		c.print(TraceLevel, args...)
	}
}

// Tracef 打印 TRACCE 日志
func (c *Console) Tracef(format string, args ...interface{}) {
	if c.level <= TraceLevel {
		c.printf(TraceLevel, format, args...)
	}
}

// Debug 打印 DEBUG 日志
func (c *Console) Debug(args ...interface{}) {
	if c.level <= DebugLevel {
		c.print(DebugLevel, args...)
	}
}

// Debugf 打印 DEBUG 日志
func (c *Console) Debugf(format string, args ...interface{}) {
	if c.level <= DebugLevel {
		c.printf(DebugLevel, format, args...)
	}
}

// Info 打印 INFO 日志
func (c *Console) Info(args ...interface{}) {
	if c.level <= InfoLevel {
		c.print(InfoLevel, args...)
	}
}

// Infof 打印 INFO 日志
func (c *Console) Infof(format string, args ...interface{}) {
	if c.level <= InfoLevel {
		c.printf(InfoLevel, format, args...)
	}
}

// Warn 打印 WARN 日志
func (c *Console) Warn(args ...interface{}) {
	if c.level <= WarnLevel {
		c.print(WarnLevel, args...)
	}
}

// Warnf 打印 WARN 日志
func (c *Console) Warnf(format string, args ...interface{}) {
	if c.level <= WarnLevel {
		c.printf(WarnLevel, format, args...)
	}
}

// Error 打印 ERROR 日志
func (c *Console) Error(args ...interface{}) {
	if c.level <= ErrorLevel {
		c.print(ErrorLevel, args...)
	}
}

// Errorf 打印 ERROR 日志
func (c *Console) Errorf(format string, args ...interface{}) {
	if c.level <= ErrorLevel {
		c.printf(ErrorLevel, format, args...)
	}
}

// Panic 打印 PANIC 日志
func (c *Console) Panic(args ...interface{}) {
	str := c.print(PanicLevel, args...)
	panic(errors.New(str))
}

// Panicf 打印 PANIC 日志
func (c *Console) Panicf(format string, args ...interface{}) {
	str := c.printf(PanicLevel, format, args...)
	panic(errors.New(str))
}

// Fatal 打印 FATAL 日志
func (c *Console) Fatal(args ...interface{}) {
	c.print(FatalLevel, args...)
	os.Exit(1)
}

// Fatalf 打印 FATAL 日志
func (c *Console) Fatalf(format string, args ...interface{}) {
	c.printf(FatalLevel, format, args...)
	os.Exit(1)
}

// Print 打印未格式化的日志
func (c *Console) Print(args ...interface{}) {
	fmt.Println(args...)
}

// Printf 打印未格式化的日志
func (c *Console) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// print
func (c *Console) print(level Level, args ...interface{}) string {
	str := fmt.Sprint(args...)
	c.log(level, str)
	return str
}

// printf
func (c *Console) printf(level Level, format string, args ...interface{}) string {
	str := fmt.Sprintf(format, args...)
	c.log(level, str)
	return str
}

// log
func (c *Console) log(level Level, msg string) {
	strLevel := strings.ToUpper(LevelToString(level))

	if level >= ErrorLevel {
		strLevel = color.Red(strLevel)
	} else if level == WarnLevel {
		strLevel = color.Yellow(strLevel)
	}

	var (
		file string
		line int
	)

	// 获取注册点信息
	for i := 2; i < 10; i++ {
		_, file0, line0, _ := runtime.Caller(i)

		// 排除 spring-core 包下面所有的非 test 文件
		if strings.Contains(file0, "/spring-logger/") {
			if !strings.HasSuffix(file0, "_test.go") {
				continue
			}
		}

		file = file0
		line = line0
		break
	}

	dir0, file0 := path.Split(file)
	file = path.Join(path.Base(dir0), file0)
	fmt.Printf("[%s] %s:%d %s\n", strLevel, file, line, msg)
}
