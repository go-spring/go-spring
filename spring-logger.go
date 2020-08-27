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

// StdLogger 标准的 Logger 接口
type StdLogger interface {
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
}

// PrefixLogger 带前缀名的 Logger 接口
type PrefixLogger interface {
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
}

// 为了平衡调用栈的深度，增加一个 StdLogger 包装类
type StdLoggerWrapper struct {
	l StdLogger
}

func (w *StdLoggerWrapper) Trace(args ...interface{}) {
	w.l.Trace(args...)
}

func (w *StdLoggerWrapper) Tracef(format string, args ...interface{}) {
	w.l.Tracef(format, args...)
}

func (w *StdLoggerWrapper) Debug(args ...interface{}) {
	w.l.Debug(args...)
}

func (w *StdLoggerWrapper) Debugf(format string, args ...interface{}) {
	w.l.Debugf(format, args...)
}

func (w *StdLoggerWrapper) Info(args ...interface{}) {
	w.l.Info(args...)
}

func (w *StdLoggerWrapper) Infof(format string, args ...interface{}) {
	w.l.Infof(format, args...)
}

func (w *StdLoggerWrapper) Warn(args ...interface{}) {
	w.l.Warn(args...)
}

func (w *StdLoggerWrapper) Warnf(format string, args ...interface{}) {
	w.l.Warnf(format, args...)
}

func (w *StdLoggerWrapper) Error(args ...interface{}) {
	w.l.Error(args...)
}

func (w *StdLoggerWrapper) Errorf(format string, args ...interface{}) {
	w.l.Errorf(format, args...)
}

func (w *StdLoggerWrapper) Panic(args ...interface{}) {
	w.l.Panic(args...)
}

func (w *StdLoggerWrapper) Panicf(format string, args ...interface{}) {
	w.l.Panicf(format, args...)
}

func (w *StdLoggerWrapper) Fatal(args ...interface{}) {
	w.l.Fatal(args...)
}

func (w *StdLoggerWrapper) Fatalf(format string, args ...interface{}) {
	w.l.Fatalf(format, args...)
}

func (w *StdLoggerWrapper) Print(args ...interface{}) {
	w.l.Print(args...)
}

func (w *StdLoggerWrapper) Printf(format string, args ...interface{}) {
	w.l.Printf(format, args...)
}
