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

package log_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/chrono"
	"github.com/go-spring/spring-base/code"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/log"
	"github.com/golang/mock/gomock"
)

func TestDefault(t *testing.T) {

	fixedTime := time.Now()
	ctx, _ := knife.New(context.Background())
	err := chrono.SetFixedTime(ctx, fixedTime)
	assert.Nil(t, err)

	log.SetDefaultContext(ctx)
	defer log.ResetToDefault()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o := log.NewMockLogger(ctrl)
	log.SetDefaultLogger(o)

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Trace("a", "=", "1")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Tracef("a=%d", 1)

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Trace(func() []interface{} {
		return log.T("a", "=", "1")
	})
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Tracef("a=%d", func() []interface{} {
		return log.T(1)
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Debug("a", "=", "1")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Debugf("a=%d", 1)

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Debug(func() []interface{} {
		return log.T("a", "=", "1")
	})
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Debugf("a=%d", func() []interface{} {
		return log.T(1)
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Info("a", "=", "1")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Infof("a=%d", 1)

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Info(func() []interface{} {
		return log.T("a", "=", "1")
	})
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Infof("a=%d", func() []interface{} {
		return log.T(1)
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Warn("a", "=", "1")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Warnf("a=%d", 1)

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Warn(func() []interface{} {
		return log.T("a", "=", "1")
	})
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Warnf("a=%d", func() []interface{} {
		return log.T(1)
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Error("a", "=", "1")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Errorf("a=%d", 1)

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Error(func() []interface{} {
		return log.T("a", "=", "1")
	})
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Errorf("a=%d", func() []interface{} {
		return log.T(1)
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Args: []interface{}{errors.New("error")}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Panic(errors.New("error"))
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Args: []interface{}{"error:404"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Panicf("error:%d", 404)

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Fatal("a", "=", "1")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	log.Fatalf("a=%d", 1)
}

func TestEntry(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace", "110110")

	ctx, _ = knife.New(ctx)
	fixedTime := time.Now()
	err := chrono.SetFixedTime(ctx, fixedTime)
	assert.Nil(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	o := log.NewMockLogger(ctrl)
	log.SetDefaultLogger(o)
	defer log.ResetToDefault()

	const tagIn = "__in"

	ctxLogger := log.WithContext(ctx)
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Args: []interface{}{"Level:", "trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Trace("Level:", "trace")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Args: []interface{}{"Level:trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Tracef("Level:%s", "trace")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Args: []interface{}{"Level:", "debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Debug("Level:", "debug")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Args: []interface{}{"Level:debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Debugf("Level:%s", "debug")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Args: []interface{}{"Level:", "info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Info("Level:", "info")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Args: []interface{}{"Level:info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Infof("Level:%s", "info")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Args: []interface{}{"Level:", "warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Warn("Level:", "warn")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Args: []interface{}{"Level:warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Warnf("Level:%s", "warn")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Args: []interface{}{"Level:", "error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Error(log.ERROR, "Level:", "error")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Args: []interface{}{"Level:error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Errorf(log.ERROR, "Level:%s", "error")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Ctx: ctx, Args: []interface{}{"Level:", "panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Panic("Level:", "panic")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Ctx: ctx, Args: []interface{}{"Level:panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Panicf("Level:%s", "panic")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Ctx: ctx, Args: []interface{}{"Level:", "fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Fatal("Level:", "fatal")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Ctx: ctx, Args: []interface{}{"Level:fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Fatalf("Level:%s", "fatal")

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Args: []interface{}{"Level:", "trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Trace(func() []interface{} {
		return log.T("Level:", "trace")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Args: []interface{}{"Level:trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Tracef("Level:%s", func() []interface{} {
		return log.T("trace")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Args: []interface{}{"Level:", "debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Debug(func() []interface{} {
		return log.T("Level:", "debug")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Args: []interface{}{"Level:debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Debugf("Level:%s", func() []interface{} {
		return log.T("debug")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Args: []interface{}{"Level:", "info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Info(func() []interface{} {
		return log.T("Level:", "info")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Args: []interface{}{"Level:info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Infof("Level:%s", func() []interface{} {
		return log.T("info")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Args: []interface{}{"Level:", "warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Warn(func() []interface{} {
		return log.T("Level:", "warn")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Args: []interface{}{"Level:warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Warnf("Level:%s", func() []interface{} {
		return log.T("warn")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Args: []interface{}{"Level:", "error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Error(log.ERROR, func() []interface{} {
		return log.T("Level:", "error")
	})

	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Args: []interface{}{"Level:error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Errorf(log.ERROR, "Level:%s", func() []interface{} {
		return log.T("error")
	})

	ctxLogger = ctxLogger.WithTag(tagIn)
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Trace("Level:", "trace")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Tracef("Level:%s", "trace")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Debug("Level:", "debug")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Debugf("Level:%s", "debug")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Info("Level:", "info")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Infof("Level:%s", "info")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Warn("Level:", "warn")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Warnf("Level:%s", "warn")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Error(log.ERROR, "Level:", "error")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Errorf(log.ERROR, "Level:%s", "error")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Panic("Level:", "panic")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Panicf("Level:%s", "panic")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Fatal("Level:", "fatal")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	ctxLogger.Fatalf("Level:%s", "fatal")

	logger := log.WithTag(tagIn)
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	logger.WithContext(ctx).Trace("Level:", "trace")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	logger.WithContext(ctx).Tracef("Level:%s", "trace")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	logger.WithContext(ctx).Debug("Level:", "debug")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	logger.WithContext(ctx).Debugf("Level:%s", "debug")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	logger.WithContext(ctx).Info("Level:", "info")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	logger.WithContext(ctx).Infof("Level:%s", "info")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	logger.WithContext(ctx).Warn("Level:", "warn")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	logger.WithContext(ctx).Warnf("Level:%s", "warn")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	logger.WithContext(ctx).Error(log.ERROR, "Level:", "error")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	logger.WithContext(ctx).Errorf(log.ERROR, "Level:%s", "error")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	logger.WithContext(ctx).Panic("Level:", "panic")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	logger.WithContext(ctx).Panicf("Level:%s", "panic")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	logger.WithContext(ctx).Fatal("Level:", "fatal")
	o.EXPECT().Level()
	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
	logger.WithContext(ctx).Fatalf("Level:%s", "fatal")
}

func TestSkip(t *testing.T) {
	func(format string, args ...interface{}) {
		log.WithSkip(1).Infof(format, args...)
	}("log skip test")
}
