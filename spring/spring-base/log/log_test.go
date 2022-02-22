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

	o := NewMockLogger(ctrl)
	RegisterDefaultLogger(o)
	defer ClearLoggers()

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: TraceLevel, args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Trace("a", "=", "1")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: TraceLevel, args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Tracef("a=%d", 1)

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: TraceLevel, args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Trace(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: TraceLevel, args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Tracef("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: DebugLevel, args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Debug("a", "=", "1")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: DebugLevel, args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Debugf("a=%d", 1)

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: DebugLevel, args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Debug(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: DebugLevel, args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Debugf("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: InfoLevel, args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Info("a", "=", "1")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: InfoLevel, args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Infof("a=%d", 1)

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: InfoLevel, args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Info(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: InfoLevel, args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Infof("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: WarnLevel, args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Warn("a", "=", "1")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: WarnLevel, args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Warnf("a=%d", 1)

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: WarnLevel, args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Warn(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: WarnLevel, args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Warnf("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: ErrorLevel, args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Error("a", "=", "1")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: ErrorLevel, args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Errorf("a=%d", 1)

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: ErrorLevel, args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Error(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: ErrorLevel, args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Errorf("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: PanicLevel, args: []interface{}{errors.New("error")}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Panic(errors.New("error"))
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: PanicLevel, args: []interface{}{"error:404"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Panicf("error:%d", 404)

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: FatalLevel, args: []interface{}{"a", "=", "1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	Fatal("a", "=", "1")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: FatalLevel, args: []interface{}{"a=1"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
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

	const tagIn = "__in"
	o := NewMockLogger(ctrl)
	RegisterDefaultLogger(o)
	defer ClearLoggers()

	ctxLogger := WithContext(ctx)
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: TraceLevel, ctx: ctx, args: []interface{}{"level:", "trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Trace("level:", "trace")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: TraceLevel, ctx: ctx, args: []interface{}{"level:trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Tracef("level:%s", "trace")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: DebugLevel, ctx: ctx, args: []interface{}{"level:", "debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Debug("level:", "debug")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: DebugLevel, ctx: ctx, args: []interface{}{"level:debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Debugf("level:%s", "debug")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: InfoLevel, ctx: ctx, args: []interface{}{"level:", "info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Info("level:", "info")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: InfoLevel, ctx: ctx, args: []interface{}{"level:info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Infof("level:%s", "info")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: WarnLevel, ctx: ctx, args: []interface{}{"level:", "warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Warn("level:", "warn")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: WarnLevel, ctx: ctx, args: []interface{}{"level:warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Warnf("level:%s", "warn")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: ErrorLevel, ctx: ctx, args: []interface{}{"level:", "error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Error(ERROR, "level:", "error")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: ErrorLevel, ctx: ctx, args: []interface{}{"level:error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Errorf(ERROR, "level:%s", "error")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: PanicLevel, ctx: ctx, args: []interface{}{"level:", "panic"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Panic("level:", "panic")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: PanicLevel, ctx: ctx, args: []interface{}{"level:panic"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Panicf("level:%s", "panic")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: FatalLevel, ctx: ctx, args: []interface{}{"level:", "fatal"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Fatal("level:", "fatal")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: FatalLevel, ctx: ctx, args: []interface{}{"level:fatal"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Fatalf("level:%s", "fatal")

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: TraceLevel, ctx: ctx, args: []interface{}{"level:", "trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Trace(func() []interface{} {
		return T("level:", "trace")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: TraceLevel, ctx: ctx, args: []interface{}{"level:trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Tracef("level:%s", func() []interface{} {
		return T("trace")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: DebugLevel, ctx: ctx, args: []interface{}{"level:", "debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Debug(func() []interface{} {
		return T("level:", "debug")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: DebugLevel, ctx: ctx, args: []interface{}{"level:debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Debugf("level:%s", func() []interface{} {
		return T("debug")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: InfoLevel, ctx: ctx, args: []interface{}{"level:", "info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Info(func() []interface{} {
		return T("level:", "info")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: InfoLevel, ctx: ctx, args: []interface{}{"level:info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Infof("level:%s", func() []interface{} {
		return T("info")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: WarnLevel, ctx: ctx, args: []interface{}{"level:", "warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Warn(func() []interface{} {
		return T("level:", "warn")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: WarnLevel, ctx: ctx, args: []interface{}{"level:warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Warnf("level:%s", func() []interface{} {
		return T("warn")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: ErrorLevel, ctx: ctx, args: []interface{}{"level:", "error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Error(ERROR, func() []interface{} {
		return T("level:", "error")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: ErrorLevel, ctx: ctx, args: []interface{}{"level:error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Errorf(ERROR, "level:%s", func() []interface{} {
		return T("error")
	})

	ctxLogger = ctxLogger.WithTag(tagIn)
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: TraceLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:", "trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Trace("level:", "trace")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: TraceLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Tracef("level:%s", "trace")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: DebugLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:", "debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Debug("level:", "debug")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: DebugLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Debugf("level:%s", "debug")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: InfoLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:", "info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Info("level:", "info")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: InfoLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Infof("level:%s", "info")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: WarnLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:", "warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Warn("level:", "warn")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: WarnLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Warnf("level:%s", "warn")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: ErrorLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:", "error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Error(ERROR, "level:", "error")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: ErrorLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Errorf(ERROR, "level:%s", "error")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: PanicLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:", "panic"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Panic("level:", "panic")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: PanicLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:panic"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Panicf("level:%s", "panic")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: FatalLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:", "fatal"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Fatal("level:", "fatal")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: FatalLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:fatal"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	ctxLogger.Fatalf("level:%s", "fatal")

	logger := WithTag(tagIn)
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: TraceLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:", "trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.WithContext(ctx).Trace("level:", "trace")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: TraceLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:trace"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.WithContext(ctx).Tracef("level:%s", "trace")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: DebugLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:", "debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.WithContext(ctx).Debug("level:", "debug")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: DebugLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:debug"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.WithContext(ctx).Debugf("level:%s", "debug")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: InfoLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:", "info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.WithContext(ctx).Info("level:", "info")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: InfoLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:info"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.WithContext(ctx).Infof("level:%s", "info")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: WarnLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:", "warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.WithContext(ctx).Warn("level:", "warn")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: WarnLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:warn"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.WithContext(ctx).Warnf("level:%s", "warn")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: ErrorLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:", "error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.WithContext(ctx).Error(ERROR, "level:", "error")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: ErrorLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:error"}, errno: ERROR, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.WithContext(ctx).Errorf(ERROR, "level:%s", "error")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: PanicLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:", "panic"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.WithContext(ctx).Panic("level:", "panic")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: PanicLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:panic"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.WithContext(ctx).Panicf("level:%s", "panic")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: FatalLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:", "fatal"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.WithContext(ctx).Fatal("level:", "fatal")
	o.EXPECT().Level()
	o.EXPECT().Print(&Message{level: FatalLevel, ctx: ctx, tag: tagIn, args: []interface{}{"level:fatal"}, file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.WithContext(ctx).Fatalf("level:%s", "fatal")
}

func TestSkip(t *testing.T) {
	func(format string, args ...interface{}) {
		WithSkip(1).Infof(format, args...)
	}("log skip test")
}
