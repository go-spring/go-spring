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
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/clock"
	"github.com/go-spring/spring-base/code"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
	"github.com/golang/mock/gomock"
)

func TestAtomicAndMutex(t *testing.T) {
	k := int32(0)

	// 直接读取，10亿次，313.234156ms。
	start := time.Now()
	for i := 0; i < 1000000000; i++ {
		j := k
		_ = j
	}
	fmt.Println(time.Since(start))

	// 原子读取，10亿次，332.547066ms。
	start = time.Now()
	for i := 0; i < 1000000000; i++ {
		j := atomic.LoadInt32(&k)
		_ = j
	}
	k = 0
	fmt.Println(time.Since(start))

	// 原子累加，10亿次，6.251721832s。
	start = time.Now()
	for i := 0; i < 100000000; i++ {
		atomic.AddInt32(&k, 1)
	}
	k = 0
	fmt.Println(time.Since(start))

	// atomic.Value，10亿次，978.367782ms。
	var v atomic.Value
	v.Store(k)
	start = time.Now()
	for i := 0; i < 1000000000; i++ {
		j := v.Load().(int32)
		_ = j
	}
	fmt.Println(time.Since(start))

	// 使用读锁，10亿次，12.758831296s。
	var mux sync.RWMutex
	start = time.Now()
	for i := 0; i < 100000000; i++ {
		mux.RLock()
		j := k
		_ = j
		mux.RUnlock()
	}
	fmt.Println(time.Since(start))
}

func TestGetLogger(t *testing.T) {
	logger := log.GetLogger("log_test")
	assert.Equal(t, logger.Name(), "log_test")
	logger = log.GetLogger()
	assert.Equal(t, logger.Name(), "github.com/go-spring/spring-base/log_test")
}

func TestRootLogger(t *testing.T) {

	logger := log.GetLogger(log.RootLoggerName)
	rootLogger := log.GetRootLogger()
	assert.Equal(t, logger, rootLogger)

	err := log.Load(`
		<Configuration>
			<Appenders>
				<ConsoleAppender name="Console">
				</ConsoleAppender>
			</Appenders>
			<Loggers>
				<Root level="INFO">
					<AppenderRef ref="Console"/>
				</Root>
			</Loggers>
		</Configuration>
	`)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			logger.Info()
		}
	}()

	time.Sleep(time.Millisecond)
	fmt.Println("done")
	time.Sleep(time.Millisecond)
}

func TestLogger(t *testing.T) {

	fixedTime := time.Now()
	ctx, _ := knife.New(context.Background())
	err := clock.SetFixedTime(ctx, fixedTime)
	assert.Nil(t, err)

	log.SetDefaultContext(ctx)
	defer log.SetDefaultContext(nil)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appender := log.NewMockAppender(ctrl)
	logger := log.NewLogger(&log.LoggerConfig{
		Level:     log.TraceLevel,
		Appenders: []log.Appender{appender},
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.InfoLevel, Args: []interface{}{"log skip test"}, File: code.File(), Line: code.Line() + 3, Time: fixedTime}).Build())
	func(format string, args ...interface{}) {
		logger.WithSkip(1).Infof(format, args...)
	}("log skip test")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.TraceLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Trace("a", "=", "1")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.TraceLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Tracef("a=%d", 1)

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.TraceLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Trace(func() []interface{} {
		return util.T("a", "=", "1")
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.TraceLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Tracef("a=%d", func() []interface{} {
		return util.T(1)
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.DebugLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Debug("a", "=", "1")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.DebugLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Debugf("a=%d", 1)

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.DebugLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Debug(func() []interface{} {
		return util.T("a", "=", "1")
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.DebugLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Debugf("a=%d", func() []interface{} {
		return util.T(1)
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.InfoLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Info("a", "=", "1")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.InfoLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Infof("a=%d", 1)

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.InfoLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Info(func() []interface{} {
		return util.T("a", "=", "1")
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.InfoLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Infof("a=%d", func() []interface{} {
		return util.T(1)
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.WarnLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Warn("a", "=", "1")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.WarnLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Warnf("a=%d", 1)

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.WarnLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Warn(func() []interface{} {
		return util.T("a", "=", "1")
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.WarnLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Warnf("a=%d", func() []interface{} {
		return util.T(1)
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.ErrorLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Error("a", "=", "1")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.ErrorLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Errorf("a=%d", 1)

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.ErrorLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Error(func() []interface{} {
		return util.T("a", "=", "1")
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.ErrorLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Errorf("a=%d", func() []interface{} {
		return util.T(1)
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.PanicLevel, Args: []interface{}{errors.New("error")}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Panic(errors.New("error"))

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.PanicLevel, Args: []interface{}{"error:404"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Panicf("error:%d", 404)

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.FatalLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Fatal("a", "=", "1")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.FatalLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	logger.Fatalf("a=%d", 1)
}

func TestEntry(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace", "110110")

	ctx, _ = knife.New(ctx)
	fixedTime := time.Now()
	err := clock.SetFixedTime(ctx, fixedTime)
	assert.Nil(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	appender := log.NewMockAppender(ctrl)
	logger := log.NewLogger(&log.LoggerConfig{
		Level:     log.TraceLevel,
		Appenders: []log.Appender{appender},
	})

	const tagIn = "__in"
	ctxLogger := logger.WithContext(ctx)

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.TraceLevel, Ctx: ctx, Args: []interface{}{"Level:", "trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Trace("Level:", "trace")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.TraceLevel, Ctx: ctx, Args: []interface{}{"Level:trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Tracef("Level:%s", "trace")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.DebugLevel, Ctx: ctx, Args: []interface{}{"Level:", "debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Debug("Level:", "debug")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.DebugLevel, Ctx: ctx, Args: []interface{}{"Level:debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Debugf("Level:%s", "debug")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.InfoLevel, Ctx: ctx, Args: []interface{}{"Level:", "info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Info("Level:", "info")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.InfoLevel, Ctx: ctx, Args: []interface{}{"Level:info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Infof("Level:%s", "info")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.WarnLevel, Ctx: ctx, Args: []interface{}{"Level:", "warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Warn("Level:", "warn")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.WarnLevel, Ctx: ctx, Args: []interface{}{"Level:warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Warnf("Level:%s", "warn")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.ErrorLevel, Ctx: ctx, Args: []interface{}{"Level:", "error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Error(log.ERROR, "Level:", "error")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.ErrorLevel, Ctx: ctx, Args: []interface{}{"Level:error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Errorf(log.ERROR, "Level:%s", "error")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.PanicLevel, Ctx: ctx, Args: []interface{}{"Level:", "panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Panic("Level:", "panic")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.PanicLevel, Ctx: ctx, Args: []interface{}{"Level:panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Panicf("Level:%s", "panic")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.FatalLevel, Ctx: ctx, Args: []interface{}{"Level:", "fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Fatal("Level:", "fatal")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.FatalLevel, Ctx: ctx, Args: []interface{}{"Level:fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Fatalf("Level:%s", "fatal")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.TraceLevel, Ctx: ctx, Args: []interface{}{"Level:", "trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Trace(func() []interface{} {
		return util.T("Level:", "trace")
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.TraceLevel, Ctx: ctx, Args: []interface{}{"Level:trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Tracef("Level:%s", func() []interface{} {
		return util.T("trace")
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.DebugLevel, Ctx: ctx, Args: []interface{}{"Level:", "debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Debug(func() []interface{} {
		return util.T("Level:", "debug")
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.DebugLevel, Ctx: ctx, Args: []interface{}{"Level:debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Debugf("Level:%s", func() []interface{} {
		return util.T("debug")
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.InfoLevel, Ctx: ctx, Args: []interface{}{"Level:", "info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Info(func() []interface{} {
		return util.T("Level:", "info")
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.InfoLevel, Ctx: ctx, Args: []interface{}{"Level:info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Infof("Level:%s", func() []interface{} {
		return util.T("info")
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.WarnLevel, Ctx: ctx, Args: []interface{}{"Level:", "warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Warn(func() []interface{} {
		return util.T("Level:", "warn")
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.WarnLevel, Ctx: ctx, Args: []interface{}{"Level:warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Warnf("Level:%s", func() []interface{} {
		return util.T("warn")
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.ErrorLevel, Ctx: ctx, Args: []interface{}{"Level:", "error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Error(log.ERROR, func() []interface{} {
		return util.T("Level:", "error")
	})

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.ErrorLevel, Ctx: ctx, Args: []interface{}{"Level:error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Errorf(log.ERROR, "Level:%s", func() []interface{} {
		return util.T("error")
	})

	ctxLogger = ctxLogger.WithTag(tagIn)

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.TraceLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Trace("Level:", "trace")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.TraceLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Tracef("Level:%s", "trace")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.DebugLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Debug("Level:", "debug")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.DebugLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Debugf("Level:%s", "debug")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.InfoLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Info("Level:", "info")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.InfoLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Infof("Level:%s", "info")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.WarnLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Warn("Level:", "warn")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.WarnLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Warnf("Level:%s", "warn")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.ErrorLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Error(log.ERROR, "Level:", "error")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.ErrorLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Errorf(log.ERROR, "Level:%s", "error")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.PanicLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Panic("Level:", "panic")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.PanicLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Panicf("Level:%s", "panic")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.FatalLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Fatal("Level:", "fatal")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.FatalLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	ctxLogger.Fatalf("Level:%s", "fatal")

	tagLogger := logger.WithTag(tagIn)

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.TraceLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	tagLogger.WithContext(ctx).Trace("Level:", "trace")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.TraceLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	tagLogger.WithContext(ctx).Tracef("Level:%s", "trace")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.DebugLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	tagLogger.WithContext(ctx).Debug("Level:", "debug")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.DebugLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	tagLogger.WithContext(ctx).Debugf("Level:%s", "debug")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.InfoLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	tagLogger.WithContext(ctx).Info("Level:", "info")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.InfoLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	tagLogger.WithContext(ctx).Infof("Level:%s", "info")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.WarnLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	tagLogger.WithContext(ctx).Warn("Level:", "warn")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.WarnLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	tagLogger.WithContext(ctx).Warnf("Level:%s", "warn")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.ErrorLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	tagLogger.WithContext(ctx).Error(log.ERROR, "Level:", "error")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.ErrorLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	tagLogger.WithContext(ctx).Errorf(log.ERROR, "Level:%s", "error")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.PanicLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	tagLogger.WithContext(ctx).Panic("Level:", "panic")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.PanicLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	tagLogger.WithContext(ctx).Panicf("Level:%s", "panic")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.FatalLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	tagLogger.WithContext(ctx).Fatal("Level:", "fatal")

	appender.EXPECT().Append((&log.MessageBuilder{Level: log.FatalLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime}).Build())
	tagLogger.WithContext(ctx).Fatalf("Level:%s", "fatal")
}
