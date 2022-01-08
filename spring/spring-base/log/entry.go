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
	GetSkip() int
	GetTag() string
	GetContext() context.Context
	GetErrNo() ErrNo
}

type BaseEntry struct {
	skip int
	tag  string
}

func (e BaseEntry) GetSkip() int {
	return e.skip
}

func (e BaseEntry) GetTag() string {
	return e.tag
}

func (e BaseEntry) GetContext() context.Context {
	return nil
}

func (e BaseEntry) GetErrNo() ErrNo {
	return nil
}

func (e BaseEntry) Skip(n int) BaseEntry {
	e.skip = n
	return e
}

func (e BaseEntry) Tag(tag string) BaseEntry {
	e.tag = tag
	return e
}

func (e BaseEntry) Ctx(ctx context.Context) CtxEntry {
	return CtxEntry{
		ctx:  ctx,
		tag:  e.tag,
		skip: e.skip,
	}
}

// Trace 输出 TRACE 级别的日志。
func (e BaseEntry) Trace(args ...interface{}) {
	output(TraceLevel, e, args)
}

// Tracef 输出 TRACE 级别的日志。
func (e BaseEntry) Tracef(format string, args ...interface{}) {
	outputf(TraceLevel, e, format, args)
}

// Debug 输出 DEBUG 级别的日志。
func (e BaseEntry) Debug(args ...interface{}) {
	output(DebugLevel, e, args)
}

// Debugf 输出 DEBUG 级别的日志。
func (e BaseEntry) Debugf(format string, args ...interface{}) {
	outputf(DebugLevel, e, format, args)
}

// Info 输出 INFO 级别的日志。
func (e BaseEntry) Info(args ...interface{}) {
	output(InfoLevel, e, args)
}

// Infof 输出 INFO 级别的日志。
func (e BaseEntry) Infof(format string, args ...interface{}) {
	outputf(InfoLevel, e, format, args)
}

// Warn 输出 WARN 级别的日志。
func (e BaseEntry) Warn(args ...interface{}) {
	output(WarnLevel, e, args)
}

// Warnf 输出 WARN 级别的日志。
func (e BaseEntry) Warnf(format string, args ...interface{}) {
	outputf(WarnLevel, e, format, args)
}

// Error 输出 ERROR 级别的日志。
func (e BaseEntry) Error(args ...interface{}) {
	output(ErrorLevel, e, args)
}

// Errorf 输出 ERROR 级别的日志。
func (e BaseEntry) Errorf(format string, args ...interface{}) {
	outputf(ErrorLevel, e, format, args)
}

// Panic 输出 PANIC 级别的日志。
func (e BaseEntry) Panic(args ...interface{}) {
	output(PanicLevel, e, args)
}

// Panicf 输出 PANIC 级别的日志。
func (e BaseEntry) Panicf(format string, args ...interface{}) {
	outputf(PanicLevel, e, format, args)
}

// Fatal 输出 FATAL 级别的日志。
func (e BaseEntry) Fatal(args ...interface{}) {
	output(FatalLevel, e, args)
}

// Fatalf 输出 FATAL 级别的日志。
func (e BaseEntry) Fatalf(format string, args ...interface{}) {
	outputf(FatalLevel, e, format, args)
}

type CtxEntry struct {
	skip  int
	tag   string
	ctx   context.Context
	errno ErrNo
}

func (e CtxEntry) GetSkip() int {
	return e.skip
}

func (e CtxEntry) GetTag() string {
	return e.tag
}

func (e CtxEntry) GetContext() context.Context {
	return e.ctx
}

func (e CtxEntry) GetErrNo() ErrNo {
	return e.errno
}

func (e CtxEntry) Skip(n int) CtxEntry {
	e.skip = n
	return e
}

func (e CtxEntry) Tag(tag string) CtxEntry {
	e.tag = tag
	return e
}

// Trace 输出 TRACE 级别的日志。
func (e CtxEntry) Trace(args ...interface{}) {
	output(TraceLevel, e, args)
}

// Tracef 输出 TRACE 级别的日志。
func (e CtxEntry) Tracef(format string, args ...interface{}) {
	outputf(TraceLevel, e, format, args)
}

// Debug 输出 DEBUG 级别的日志。
func (e CtxEntry) Debug(args ...interface{}) {
	output(DebugLevel, e, args)
}

// Debugf 输出 DEBUG 级别的日志。
func (e CtxEntry) Debugf(format string, args ...interface{}) {
	outputf(DebugLevel, e, format, args)
}

// Info 输出 INFO 级别的日志。
func (e CtxEntry) Info(args ...interface{}) {
	output(InfoLevel, e, args)
}

// Infof 输出 INFO 级别的日志。
func (e CtxEntry) Infof(format string, args ...interface{}) {
	outputf(InfoLevel, e, format, args)
}

// Warn 输出 WARN 级别的日志。
func (e CtxEntry) Warn(args ...interface{}) {
	output(WarnLevel, e, args)
}

// Warnf 输出 WARN 级别的日志。
func (e CtxEntry) Warnf(format string, args ...interface{}) {
	outputf(WarnLevel, e, format, args)
}

// Error 输出 ERROR 级别的日志。
func (e CtxEntry) Error(errno ErrNo, args ...interface{}) {
	e.errno = errno
	output(ErrorLevel, e, args)
}

// Errorf 输出 ERROR 级别的日志。
func (e CtxEntry) Errorf(errno ErrNo, format string, args ...interface{}) {
	e.errno = errno
	outputf(ErrorLevel, e, format, args)
}

// Panic 输出 PANIC 级别的日志。
func (e CtxEntry) Panic(args ...interface{}) {
	output(PanicLevel, e, args)
}

// Panicf 输出 PANIC 级别的日志。
func (e CtxEntry) Panicf(format string, args ...interface{}) {
	outputf(PanicLevel, e, format, args)
}

// Fatal 输出 FATAL 级别的日志。
func (e CtxEntry) Fatal(args ...interface{}) {
	output(FatalLevel, e, args)
}

// Fatalf 输出 FATAL 级别的日志。
func (e CtxEntry) Fatalf(format string, args ...interface{}) {
	outputf(FatalLevel, e, format, args)
}
