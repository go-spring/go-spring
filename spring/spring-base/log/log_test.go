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
	"errors"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/chrono"
	"github.com/go-spring/spring-base/code"
	"github.com/go-spring/spring-base/knife"
	"github.com/golang/mock/gomock"
)

func TestDefault(t *testing.T) {

	fixedTime := time.Now()
	ctx, _ := knife.New(context.Background())
	err := chrono.SetFixedTime(ctx, fixedTime)
	assert.Nil(t, err)

	defaultContext = ctx
	defer func() { defaultContext = nil }()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o := NewMockOutput(ctrl)

	SetLevel(TraceLevel)
	SetOutput(o)
	defer Reset()

	o.EXPECT().Do(TraceLevel, &Message{args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Trace("a", "=", "1")
	o.EXPECT().Do(TraceLevel, &Message{args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Tracef("a=%d", 1)

	o.EXPECT().Do(TraceLevel, &Message{args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Trace(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Do(TraceLevel, &Message{args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Tracef("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Do(DebugLevel, &Message{args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Debug("a", "=", "1")
	o.EXPECT().Do(DebugLevel, &Message{args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Debugf("a=%d", 1)

	o.EXPECT().Do(DebugLevel, &Message{args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Debug(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Do(DebugLevel, &Message{args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Debugf("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Do(InfoLevel, &Message{args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Info("a", "=", "1")
	o.EXPECT().Do(InfoLevel, &Message{args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Infof("a=%d", 1)

	o.EXPECT().Do(InfoLevel, &Message{args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Info(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Do(InfoLevel, &Message{args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Infof("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Do(WarnLevel, &Message{args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Warn("a", "=", "1")
	o.EXPECT().Do(WarnLevel, &Message{args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Warnf("a=%d", 1)

	o.EXPECT().Do(WarnLevel, &Message{args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Warn(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Do(WarnLevel, &Message{args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Warnf("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Do(ErrorLevel, &Message{args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Error("a", "=", "1")
	o.EXPECT().Do(ErrorLevel, &Message{args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Errorf("a=%d", 1)

	o.EXPECT().Do(ErrorLevel, &Message{args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Error(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Do(ErrorLevel, &Message{args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Errorf("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Do(PanicLevel, &Message{args: []interface{}{errors.New("error")}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Panic(errors.New("error"))
	o.EXPECT().Do(PanicLevel, &Message{args: []interface{}{"error:404"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Panicf("error:%d", 404)

	o.EXPECT().Do(FatalLevel, &Message{args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Fatal("a", "=", "1")
	o.EXPECT().Do(FatalLevel, &Message{args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Fatalf("a=%d", 1)
}

func TestEntry(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace", "110110")

	ctx, _ = knife.New(ctx)
	fixedTime := time.Now()
	err := chrono.SetFixedTime(ctx, fixedTime)
	assert.Nil(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o := NewMockOutput(ctrl)

	SetLevel(TraceLevel)
	SetOutput(o)
	defer Reset()

	ctxLogger := Ctx(ctx)
	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, args: []interface{}{"level:", "trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Trace("level:", "trace")
	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, args: []interface{}{"level:trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Tracef("level:%s", "trace")
	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, args: []interface{}{"level:", "debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Debug("level:", "debug")
	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, args: []interface{}{"level:debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Debugf("level:%s", "debug")
	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, args: []interface{}{"level:", "info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Info("level:", "info")
	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, args: []interface{}{"level:info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Infof("level:%s", "info")
	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, args: []interface{}{"level:", "warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Warn("level:", "warn")
	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, args: []interface{}{"level:warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Warnf("level:%s", "warn")
	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, args: []interface{}{"level:", "error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Error(ERROR, "level:", "error")
	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, args: []interface{}{"level:error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Errorf(ERROR, "level:%s", "error")
	o.EXPECT().Do(PanicLevel, &Message{ctx: ctx, args: []interface{}{"level:", "panic"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Panic("level:", "panic")
	o.EXPECT().Do(PanicLevel, &Message{ctx: ctx, args: []interface{}{"level:panic"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Panicf("level:%s", "panic")
	o.EXPECT().Do(FatalLevel, &Message{ctx: ctx, args: []interface{}{"level:", "fatal"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Fatal("level:", "fatal")
	o.EXPECT().Do(FatalLevel, &Message{ctx: ctx, args: []interface{}{"level:fatal"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Fatalf("level:%s", "fatal")

	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, args: []interface{}{"level:", "trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Trace(func() []interface{} {
		return T("level:", "trace")
	})

	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, args: []interface{}{"level:trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Tracef("level:%s", func() []interface{} {
		return T("trace")
	})

	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, args: []interface{}{"level:", "debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Debug(func() []interface{} {
		return T("level:", "debug")
	})

	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, args: []interface{}{"level:debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Debugf("level:%s", func() []interface{} {
		return T("debug")
	})

	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, args: []interface{}{"level:", "info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Info(func() []interface{} {
		return T("level:", "info")
	})

	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, args: []interface{}{"level:info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Infof("level:%s", func() []interface{} {
		return T("info")
	})

	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, args: []interface{}{"level:", "warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Warn(func() []interface{} {
		return T("level:", "warn")
	})

	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, args: []interface{}{"level:warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Warnf("level:%s", func() []interface{} {
		return T("warn")
	})

	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, args: []interface{}{"level:", "error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Error(ERROR, func() []interface{} {
		return T("level:", "error")
	})

	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, args: []interface{}{"level:error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Errorf(ERROR, "level:%s", func() []interface{} {
		return T("error")
	})

	ctxLogger = ctxLogger.Tag("__in")
	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:", "trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Trace("level:", "trace")
	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Tracef("level:%s", "trace")
	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:", "debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Debug("level:", "debug")
	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Debugf("level:%s", "debug")
	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:", "info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Info("level:", "info")
	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Infof("level:%s", "info")
	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:", "warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Warn("level:", "warn")
	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Warnf("level:%s", "warn")
	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:", "error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Error(ERROR, "level:", "error")
	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Errorf(ERROR, "level:%s", "error")
	o.EXPECT().Do(PanicLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:", "panic"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Panic("level:", "panic")
	o.EXPECT().Do(PanicLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:panic"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Panicf("level:%s", "panic")
	o.EXPECT().Do(FatalLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:", "fatal"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Fatal("level:", "fatal")
	o.EXPECT().Do(FatalLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:fatal"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Fatalf("level:%s", "fatal")

	logger := Tag("__in")
	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:", "trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Trace("level:", "trace")
	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Tracef("level:%s", "trace")
	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:", "debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Debug("level:", "debug")
	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Debugf("level:%s", "debug")
	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:", "info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Info("level:", "info")
	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Infof("level:%s", "info")
	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:", "warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Warn("level:", "warn")
	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Warnf("level:%s", "warn")
	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:", "error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Error(ERROR, "level:", "error")
	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Errorf(ERROR, "level:%s", "error")
	o.EXPECT().Do(PanicLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:", "panic"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Panic("level:", "panic")
	o.EXPECT().Do(PanicLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:panic"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Panicf("level:%s", "panic")
	o.EXPECT().Do(FatalLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:", "fatal"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Fatal("level:", "fatal")
	o.EXPECT().Do(FatalLevel, &Message{ctx: ctx, tag: "__in", args: []interface{}{"level:fatal"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Fatalf("level:%s", "fatal")
}

func TestSkip(t *testing.T) {
	func(format string, args ...interface{}) {
		Skip(1).Infof(format, args...)
	}("log skip test")
}
