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
)

type Entry interface {
	Logger() *Logger
	Skip() int
	Tag() string
	Errno() Errno
	Context() context.Context
}

type BaseEntry struct {
	logger *Logger
	tag    string
	skip   int
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

// Trace outputs log with level TraceLevel.
func (e BaseEntry) Trace(args ...interface{}) {
	printf(TraceLevel, &e, "", args)
}

// Tracef outputs log with level TraceLevel.
func (e BaseEntry) Tracef(format string, args ...interface{}) {
	printf(TraceLevel, &e, format, args)
}

// Debug outputs log with level DebugLevel.
func (e BaseEntry) Debug(args ...interface{}) {
	printf(DebugLevel, &e, "", args)
}

// Debugf outputs log with level DebugLevel.
func (e BaseEntry) Debugf(format string, args ...interface{}) {
	printf(DebugLevel, &e, format, args)
}

// Info outputs log with level InfoLevel.
func (e BaseEntry) Info(args ...interface{}) {
	printf(InfoLevel, &e, "", args)
}

// Infof outputs log with level InfoLevel.
func (e BaseEntry) Infof(format string, args ...interface{}) {
	printf(InfoLevel, &e, format, args)
}

// Warn outputs log with level WarnLevel.
func (e BaseEntry) Warn(args ...interface{}) {
	printf(WarnLevel, &e, "", args)
}

// Warnf outputs log with level WarnLevel.
func (e BaseEntry) Warnf(format string, args ...interface{}) {
	printf(WarnLevel, &e, format, args)
}

// Error outputs log with level ErrorLevel.
func (e BaseEntry) Error(args ...interface{}) {
	printf(ErrorLevel, &e, "", args)
}

// Errorf outputs log with level ErrorLevel.
func (e BaseEntry) Errorf(format string, args ...interface{}) {
	printf(ErrorLevel, &e, format, args)
}

// Panic outputs log with level PanicLevel.
func (e BaseEntry) Panic(args ...interface{}) {
	printf(PanicLevel, &e, "", args)
}

// Panicf outputs log with level PanicLevel.
func (e BaseEntry) Panicf(format string, args ...interface{}) {
	printf(PanicLevel, &e, format, args)
}

// Fatal outputs log with level FatalLevel.
func (e BaseEntry) Fatal(args ...interface{}) {
	printf(FatalLevel, &e, "", args)
}

// Fatalf outputs log with level FatalLevel.
func (e BaseEntry) Fatalf(format string, args ...interface{}) {
	printf(FatalLevel, &e, format, args)
}

type CtxEntry struct {
	ctx    context.Context
	errno  Errno
	logger *Logger
	tag    string
	skip   int
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

// Trace outputs log with level TraceLevel.
func (e CtxEntry) Trace(args ...interface{}) {
	printf(TraceLevel, &e, "", args)
}

// Tracef outputs log with level TraceLevel.
func (e CtxEntry) Tracef(format string, args ...interface{}) {
	printf(TraceLevel, &e, format, args)
}

// Debug outputs log with level DebugLevel.
func (e CtxEntry) Debug(args ...interface{}) {
	printf(DebugLevel, &e, "", args)
}

// Debugf outputs log with level DebugLevel.
func (e CtxEntry) Debugf(format string, args ...interface{}) {
	printf(DebugLevel, &e, format, args)
}

// Info outputs log with level InfoLevel.
func (e CtxEntry) Info(args ...interface{}) {
	printf(InfoLevel, &e, "", args)
}

// Infof outputs log with level InfoLevel.
func (e CtxEntry) Infof(format string, args ...interface{}) {
	printf(InfoLevel, &e, format, args)
}

// Warn outputs log with level WarnLevel.
func (e CtxEntry) Warn(args ...interface{}) {
	printf(WarnLevel, &e, "", args)
}

// Warnf outputs log with level WarnLevel.
func (e CtxEntry) Warnf(format string, args ...interface{}) {
	printf(WarnLevel, &e, format, args)
}

// Error outputs log with level ErrorLevel.
func (e CtxEntry) Error(errno Errno, args ...interface{}) {
	e.errno = errno
	printf(ErrorLevel, &e, "", args)
}

// Errorf outputs log with level ErrorLevel.
func (e CtxEntry) Errorf(errno Errno, format string, args ...interface{}) {
	e.errno = errno
	printf(ErrorLevel, &e, format, args)
}

// Panic outputs log with level PanicLevel.
func (e CtxEntry) Panic(args ...interface{}) {
	printf(PanicLevel, &e, "", args)
}

// Panicf outputs log with level PanicLevel.
func (e CtxEntry) Panicf(format string, args ...interface{}) {
	printf(PanicLevel, &e, format, args)
}

// Fatal outputs log with level FatalLevel.
func (e CtxEntry) Fatal(args ...interface{}) {
	printf(FatalLevel, &e, "", args)
}

// Fatalf outputs log with level FatalLevel.
func (e CtxEntry) Fatalf(format string, args ...interface{}) {
	printf(FatalLevel, &e, format, args)
}
