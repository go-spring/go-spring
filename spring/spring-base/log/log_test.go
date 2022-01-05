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
	ctx := knife.New(context.Background())
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

	o.EXPECT().Do(TraceLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Trace("a", "=", "1")
	o.EXPECT().Do(TraceLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Tracef("a=%d", 1)

	o.EXPECT().Do(TraceLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Trace(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Do(TraceLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Tracef("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Do(DebugLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Debug("a", "=", "1")
	o.EXPECT().Do(DebugLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Debugf("a=%d", 1)

	o.EXPECT().Do(DebugLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Debug(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Do(DebugLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Debugf("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Do(InfoLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Info("a", "=", "1")
	o.EXPECT().Do(InfoLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Infof("a=%d", 1)

	o.EXPECT().Do(InfoLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Info(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Do(InfoLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Infof("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Do(WarnLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Warn("a", "=", "1")
	o.EXPECT().Do(WarnLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Warnf("a=%d", 1)

	o.EXPECT().Do(WarnLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Warn(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Do(WarnLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Warnf("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Do(ErrorLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Error("a", "=", "1")
	o.EXPECT().Do(ErrorLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Errorf("a=%d", 1)

	o.EXPECT().Do(ErrorLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Error(func() []interface{} {
		return T("a", "=", "1")
	})
	o.EXPECT().Do(ErrorLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Errorf("a=%d", func() []interface{} {
		return T(1)
	})

	o.EXPECT().Do(PanicLevel, &Message{text: "error", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Panic(errors.New("error"))
	o.EXPECT().Do(PanicLevel, &Message{text: "error:404", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Panicf("error:%d", 404)

	o.EXPECT().Do(FatalLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Fatal("a", "=", "1")
	o.EXPECT().Do(FatalLevel, &Message{text: "a=1", file: code.File(), line: code.Line() + 1, time: fixedTime})
	Fatalf("a=%d", 1)
}

func TestEntry(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace", "110110")

	ctx = knife.New(ctx)
	fixedTime := time.Now()
	err := chrono.SetFixedTime(ctx, fixedTime)
	assert.Nil(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o := NewMockOutput(ctrl)

	SetLevel(TraceLevel)
	SetOutput(o)
	defer Reset()

	logger := Ctx(ctx)
	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, text: "level:trace", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Trace("level:", "trace")
	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, text: "level:trace", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Tracef("level:%s", "trace")
	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, text: "level:debug", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Debug("level:", "debug")
	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, text: "level:debug", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Debugf("level:%s", "debug")
	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, text: "level:info", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Info("level:", "info")
	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, text: "level:info", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Infof("level:%s", "info")
	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, text: "level:warn", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Warn("level:", "warn")
	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, text: "level:warn", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Warnf("level:%s", "warn")
	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, text: "level:error", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Error("level:", "error")
	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, text: "level:error", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Errorf("level:%s", "error")
	o.EXPECT().Do(PanicLevel, &Message{ctx: ctx, text: "level:panic", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Panic("level:", "panic")
	o.EXPECT().Do(PanicLevel, &Message{ctx: ctx, text: "level:panic", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Panicf("level:%s", "panic")
	o.EXPECT().Do(FatalLevel, &Message{ctx: ctx, text: "level:fatal", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Fatal("level:", "fatal")
	o.EXPECT().Do(FatalLevel, &Message{ctx: ctx, text: "level:fatal", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Fatalf("level:%s", "fatal")

	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, text: "level:trace", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Trace(func() []interface{} {
		return T("level:", "trace")
	})

	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, text: "level:trace", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Tracef("level:%s", func() []interface{} {
		return T("trace")
	})

	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, text: "level:debug", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Debug(func() []interface{} {
		return T("level:", "debug")
	})

	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, text: "level:debug", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Debugf("level:%s", func() []interface{} {
		return T("debug")
	})

	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, text: "level:info", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Info(func() []interface{} {
		return T("level:", "info")
	})

	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, text: "level:info", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Infof("level:%s", func() []interface{} {
		return T("info")
	})

	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, text: "level:warn", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Warn(func() []interface{} {
		return T("level:", "warn")
	})

	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, text: "level:warn", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Warnf("level:%s", func() []interface{} {
		return T("warn")
	})

	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, text: "level:error", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Error(func() []interface{} {
		return T("level:", "error")
	})

	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, text: "level:error", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Errorf("level:%s", func() []interface{} {
		return T("error")
	})

	logger = logger.Tag("__in")
	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, tag: "__in", text: "level:trace", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Trace("level:", "trace")
	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, tag: "__in", text: "level:trace", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Tracef("level:%s", "trace")
	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, tag: "__in", text: "level:debug", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Debug("level:", "debug")
	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, tag: "__in", text: "level:debug", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Debugf("level:%s", "debug")
	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, tag: "__in", text: "level:info", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Info("level:", "info")
	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, tag: "__in", text: "level:info", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Infof("level:%s", "info")
	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, tag: "__in", text: "level:warn", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Warn("level:", "warn")
	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, tag: "__in", text: "level:warn", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Warnf("level:%s", "warn")
	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, tag: "__in", text: "level:error", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Error("level:", "error")
	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, tag: "__in", text: "level:error", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Errorf("level:%s", "error")
	o.EXPECT().Do(PanicLevel, &Message{ctx: ctx, tag: "__in", text: "level:panic", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Panic("level:", "panic")
	o.EXPECT().Do(PanicLevel, &Message{ctx: ctx, tag: "__in", text: "level:panic", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Panicf("level:%s", "panic")
	o.EXPECT().Do(FatalLevel, &Message{ctx: ctx, tag: "__in", text: "level:fatal", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Fatal("level:", "fatal")
	o.EXPECT().Do(FatalLevel, &Message{ctx: ctx, tag: "__in", text: "level:fatal", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Fatalf("level:%s", "fatal")

	logger = Tag("__in")
	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, tag: "__in", text: "level:trace", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Trace("level:", "trace")
	o.EXPECT().Do(TraceLevel, &Message{ctx: ctx, tag: "__in", text: "level:trace", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Tracef("level:%s", "trace")
	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, tag: "__in", text: "level:debug", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Debug("level:", "debug")
	o.EXPECT().Do(DebugLevel, &Message{ctx: ctx, tag: "__in", text: "level:debug", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Debugf("level:%s", "debug")
	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, tag: "__in", text: "level:info", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Info("level:", "info")
	o.EXPECT().Do(InfoLevel, &Message{ctx: ctx, tag: "__in", text: "level:info", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Infof("level:%s", "info")
	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, tag: "__in", text: "level:warn", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Warn("level:", "warn")
	o.EXPECT().Do(WarnLevel, &Message{ctx: ctx, tag: "__in", text: "level:warn", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Warnf("level:%s", "warn")
	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, tag: "__in", text: "level:error", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Error("level:", "error")
	o.EXPECT().Do(ErrorLevel, &Message{ctx: ctx, tag: "__in", text: "level:error", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Errorf("level:%s", "error")
	o.EXPECT().Do(PanicLevel, &Message{ctx: ctx, tag: "__in", text: "level:panic", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Panic("level:", "panic")
	o.EXPECT().Do(PanicLevel, &Message{ctx: ctx, tag: "__in", text: "level:panic", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Panicf("level:%s", "panic")
	o.EXPECT().Do(FatalLevel, &Message{ctx: ctx, tag: "__in", text: "level:fatal", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Fatal("level:", "fatal")
	o.EXPECT().Do(FatalLevel, &Message{ctx: ctx, tag: "__in", text: "level:fatal", file: code.File(), line: code.Line() + 1, time: fixedTime})
	logger.Ctx(ctx).Fatalf("level:%s", "fatal")
}

func TestSkip(t *testing.T) {
	func(format string, args ...interface{}) {
		Skip(1).Infof(format, args...)
	}("log skip test")
}
