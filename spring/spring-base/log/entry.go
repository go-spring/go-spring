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

import "context"

type Entry interface {
	Logger() *Logger
	Skip() int
	Tag() string
	Errno() Errno
	Context() context.Context
}

type BaseEntry struct {
	logger *Logger
	skip   int
	tag    string
}

func (e *BaseEntry) Logger() *Logger {
	return e.logger
}

func (e *BaseEntry) Skip() int {
	return e.skip
}

func (e *BaseEntry) Tag() string {
	return e.tag
}

func (e *BaseEntry) Errno() Errno {
	return nil
}

func (e *BaseEntry) Context() context.Context {
	return nil
}

func (e BaseEntry) WithSkip(n int) BaseEntry {
	e.skip = n
	return e
}

func (e BaseEntry) WithTag(tag string) BaseEntry {
	e.tag = tag
	return e
}

func (e BaseEntry) WithContext(ctx context.Context) CtxEntry {
	return CtxEntry{
		logger: e.logger,
		skip:   e.skip,
		tag:    e.tag,
		ctx:    ctx,
	}
}

// Trace 输出 TRACE 级别的日志。
func (e BaseEntry) Trace(args ...interface{}) {
	printf(TraceLevel, &e, "", args)
}

// Tracef 输出 TRACE 级别的日志。
func (e BaseEntry) Tracef(format string, args ...interface{}) {
	printf(TraceLevel, &e, format, args)
}

// Debug 输出 DEBUG 级别的日志。
func (e BaseEntry) Debug(args ...interface{}) {
	printf(DebugLevel, &e, "", args)
}

// Debugf 输出 DEBUG 级别的日志。
func (e BaseEntry) Debugf(format string, args ...interface{}) {
	printf(DebugLevel, &e, format, args)
}

// Info 输出 INFO 级别的日志。
func (e BaseEntry) Info(args ...interface{}) {
	printf(InfoLevel, &e, "", args)
}

// Infof 输出 INFO 级别的日志。
func (e BaseEntry) Infof(format string, args ...interface{}) {
	printf(InfoLevel, &e, format, args)
}

// Warn 输出 WARN 级别的日志。
func (e BaseEntry) Warn(args ...interface{}) {
	printf(WarnLevel, &e, "", args)
}

// Warnf 输出 WARN 级别的日志。
func (e BaseEntry) Warnf(format string, args ...interface{}) {
	printf(WarnLevel, &e, format, args)
}

// Error 输出 ERROR 级别的日志。
func (e BaseEntry) Error(args ...interface{}) {
	printf(ErrorLevel, &e, "", args)
}

// Errorf 输出 ERROR 级别的日志。
func (e BaseEntry) Errorf(format string, args ...interface{}) {
	printf(ErrorLevel, &e, format, args)
}

// Panic 输出 PANIC 级别的日志。
func (e BaseEntry) Panic(args ...interface{}) {
	printf(PanicLevel, &e, "", args)
}

// Panicf 输出 PANIC 级别的日志。
func (e BaseEntry) Panicf(format string, args ...interface{}) {
	printf(PanicLevel, &e, format, args)
}

// Fatal 输出 FATAL 级别的日志。
func (e BaseEntry) Fatal(args ...interface{}) {
	printf(FatalLevel, &e, "", args)
}

// Fatalf 输出 FATAL 级别的日志。
func (e BaseEntry) Fatalf(format string, args ...interface{}) {
	printf(FatalLevel, &e, format, args)
}

type CtxEntry struct {
	logger *Logger
	skip   int
	tag    string
	ctx    context.Context
	errno  Errno
}

func (e *CtxEntry) Logger() *Logger {
	return e.logger
}

func (e *CtxEntry) Skip() int {
	return e.skip
}

func (e *CtxEntry) Tag() string {
	return e.tag
}

func (e *CtxEntry) Errno() Errno {
	return e.errno
}

func (e *CtxEntry) Context() context.Context {
	return e.ctx
}

func (e CtxEntry) WithSkip(n int) CtxEntry {
	e.skip = n
	return e
}

func (e CtxEntry) WithTag(tag string) CtxEntry {
	e.tag = tag
	return e
}

// Trace 输出 TRACE 级别的日志。
func (e CtxEntry) Trace(args ...interface{}) {
	printf(TraceLevel, &e, "", args)
}

// Tracef 输出 TRACE 级别的日志。
func (e CtxEntry) Tracef(format string, args ...interface{}) {
	printf(TraceLevel, &e, format, args)
}

// Debug 输出 DEBUG 级别的日志。
func (e CtxEntry) Debug(args ...interface{}) {
	printf(DebugLevel, &e, "", args)
}

// Debugf 输出 DEBUG 级别的日志。
func (e CtxEntry) Debugf(format string, args ...interface{}) {
	printf(DebugLevel, &e, format, args)
}

// Info 输出 INFO 级别的日志。
func (e CtxEntry) Info(args ...interface{}) {
	printf(InfoLevel, &e, "", args)
}

// Infof 输出 INFO 级别的日志。
func (e CtxEntry) Infof(format string, args ...interface{}) {
	printf(InfoLevel, &e, format, args)
}

// Warn 输出 WARN 级别的日志。
func (e CtxEntry) Warn(args ...interface{}) {
	printf(WarnLevel, &e, "", args)
}

// Warnf 输出 WARN 级别的日志。
func (e CtxEntry) Warnf(format string, args ...interface{}) {
	printf(WarnLevel, &e, format, args)
}

// Error 输出 ERROR 级别的日志。
func (e CtxEntry) Error(errno Errno, args ...interface{}) {
	e.errno = errno
	printf(ErrorLevel, &e, "", args)
}

// Errorf 输出 ERROR 级别的日志。
func (e CtxEntry) Errorf(errno Errno, format string, args ...interface{}) {
	e.errno = errno
	printf(ErrorLevel, &e, format, args)
}

// Panic 输出 PANIC 级别的日志。
func (e CtxEntry) Panic(args ...interface{}) {
	printf(PanicLevel, &e, "", args)
}

// Panicf 输出 PANIC 级别的日志。
func (e CtxEntry) Panicf(format string, args ...interface{}) {
	printf(PanicLevel, &e, format, args)
}

// Fatal 输出 FATAL 级别的日志。
func (e CtxEntry) Fatal(args ...interface{}) {
	printf(FatalLevel, &e, "", args)
}

// Fatalf 输出 FATAL 级别的日志。
func (e CtxEntry) Fatalf(format string, args ...interface{}) {
	printf(FatalLevel, &e, format, args)
}
