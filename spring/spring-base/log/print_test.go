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
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-spring/spring-base/log"
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

func TestLogger(t *testing.T) {
	logger := log.GetRootLogger()
	go func() {
		for {
			logger.Info()
		}
	}()
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
	time.Sleep(time.Millisecond)
	fmt.Println("done")
	time.Sleep(time.Millisecond)
}

//
//func TestDefault(t *testing.T) {
//
//	fixedTime := time.Now()
//	ctx, _ := knife.New(context.Background())
//	err := clock.SetFixedTime(ctx, fixedTime)
//	assert.Nil(t, err)
//
//	log.SetDefaultContext(ctx)
//	defer log.ResetToDefault()
//
//	ctrl := gomock.NewController(t)
//	defer ctrl.Finish()
//
//	o := log.NewMockLogger(ctrl)
//	log.SetDefaultLogger(o)
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Trace("a", "=", "1")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Tracef("a=%d", 1)
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Trace(func() []interface{} {
//		return util.T("a", "=", "1")
//	})
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Tracef("a=%d", func() []interface{} {
//		return util.T(1)
//	})
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Debug("a", "=", "1")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Debugf("a=%d", 1)
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Debug(func() []interface{} {
//		return util.T("a", "=", "1")
//	})
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Debugf("a=%d", func() []interface{} {
//		return util.T(1)
//	})
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Info("a", "=", "1")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Infof("a=%d", 1)
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Info(func() []interface{} {
//		return util.T("a", "=", "1")
//	})
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Infof("a=%d", func() []interface{} {
//		return util.T(1)
//	})
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Warn("a", "=", "1")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Warnf("a=%d", 1)
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Warn(func() []interface{} {
//		return util.T("a", "=", "1")
//	})
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Warnf("a=%d", func() []interface{} {
//		return util.T(1)
//	})
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Error("a", "=", "1")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Errorf("a=%d", 1)
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Error(func() []interface{} {
//		return util.T("a", "=", "1")
//	})
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Errorf("a=%d", func() []interface{} {
//		return util.T(1)
//	})
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Args: []interface{}{errors.New("error")}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Panic(errors.New("error"))
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Args: []interface{}{"error:404"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Panicf("error:%d", 404)
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Args: []interface{}{"a", "=", "1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Fatal("a", "=", "1")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Args: []interface{}{"a=1"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	log.Fatalf("a=%d", 1)
//}
//
//func TestEntry(t *testing.T) {
//	ctx := context.WithValue(context.Background(), "trace", "110110")
//
//	ctx, _ = knife.New(ctx)
//	fixedTime := time.Now()
//	err := clock.SetFixedTime(ctx, fixedTime)
//	assert.Nil(t, err)
//
//	ctrl := gomock.NewController(t)
//	defer ctrl.Finish()
//
//	o := log.NewMockLogger(ctrl)
//	log.SetDefaultLogger(o)
//	defer log.ResetToDefault()
//
//	const tagIn = "__in"
//
//	ctxLogger := log.WithContext(ctx)
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Args: []interface{}{"Level:", "trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Trace("Level:", "trace")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Args: []interface{}{"Level:trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Tracef("Level:%s", "trace")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Args: []interface{}{"Level:", "debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Debug("Level:", "debug")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Args: []interface{}{"Level:debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Debugf("Level:%s", "debug")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Args: []interface{}{"Level:", "info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Info("Level:", "info")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Args: []interface{}{"Level:info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Infof("Level:%s", "info")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Args: []interface{}{"Level:", "warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Warn("Level:", "warn")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Args: []interface{}{"Level:warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Warnf("Level:%s", "warn")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Args: []interface{}{"Level:", "error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Error(log.ERROR, "Level:", "error")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Args: []interface{}{"Level:error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Errorf(log.ERROR, "Level:%s", "error")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Ctx: ctx, Args: []interface{}{"Level:", "panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Panic("Level:", "panic")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Ctx: ctx, Args: []interface{}{"Level:panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Panicf("Level:%s", "panic")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Ctx: ctx, Args: []interface{}{"Level:", "fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Fatal("Level:", "fatal")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Ctx: ctx, Args: []interface{}{"Level:fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Fatalf("Level:%s", "fatal")
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Args: []interface{}{"Level:", "trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Trace(func() []interface{} {
//		return util.T("Level:", "trace")
//	})
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Args: []interface{}{"Level:trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Tracef("Level:%s", func() []interface{} {
//		return util.T("trace")
//	})
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Args: []interface{}{"Level:", "debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Debug(func() []interface{} {
//		return util.T("Level:", "debug")
//	})
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Args: []interface{}{"Level:debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Debugf("Level:%s", func() []interface{} {
//		return util.T("debug")
//	})
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Args: []interface{}{"Level:", "info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Info(func() []interface{} {
//		return util.T("Level:", "info")
//	})
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Args: []interface{}{"Level:info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Infof("Level:%s", func() []interface{} {
//		return util.T("info")
//	})
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Args: []interface{}{"Level:", "warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Warn(func() []interface{} {
//		return util.T("Level:", "warn")
//	})
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Args: []interface{}{"Level:warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Warnf("Level:%s", func() []interface{} {
//		return util.T("warn")
//	})
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Args: []interface{}{"Level:", "error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Error(log.ERROR, func() []interface{} {
//		return util.T("Level:", "error")
//	})
//
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Args: []interface{}{"Level:error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Errorf(log.ERROR, "Level:%s", func() []interface{} {
//		return util.T("error")
//	})
//
//	ctxLogger = ctxLogger.WithTag(tagIn)
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Trace("Level:", "trace")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Tracef("Level:%s", "trace")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Debug("Level:", "debug")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Debugf("Level:%s", "debug")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Info("Level:", "info")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Infof("Level:%s", "info")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Warn("Level:", "warn")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Warnf("Level:%s", "warn")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Error(log.ERROR, "Level:", "error")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Errorf(log.ERROR, "Level:%s", "error")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Panic("Level:", "panic")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Panicf("Level:%s", "panic")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Fatal("Level:", "fatal")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	ctxLogger.Fatalf("Level:%s", "fatal")
//
//	logger := log.WithTag(tagIn)
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	logger.WithContext(ctx).Trace("Level:", "trace")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.TraceLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:trace"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	logger.WithContext(ctx).Tracef("Level:%s", "trace")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	logger.WithContext(ctx).Debug("Level:", "debug")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.DebugLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:debug"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	logger.WithContext(ctx).Debugf("Level:%s", "debug")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	logger.WithContext(ctx).Info("Level:", "info")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.InfoLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:info"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	logger.WithContext(ctx).Infof("Level:%s", "info")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	logger.WithContext(ctx).Warn("Level:", "warn")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.WarnLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:warn"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	logger.WithContext(ctx).Warnf("Level:%s", "warn")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	logger.WithContext(ctx).Error(log.ERROR, "Level:", "error")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.ErrorLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:error"}, Errno: log.ERROR, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	logger.WithContext(ctx).Errorf(log.ERROR, "Level:%s", "error")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	logger.WithContext(ctx).Panic("Level:", "panic")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.PanicLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:panic"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	logger.WithContext(ctx).Panicf("Level:%s", "panic")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:", "fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	logger.WithContext(ctx).Fatal("Level:", "fatal")
//	o.EXPECT().Level()
//	o.EXPECT().Print(&log.Message{Level: log.FatalLevel, Ctx: ctx, Tag: tagIn, Args: []interface{}{"Level:fatal"}, File: code.File(), Line: code.Line() + 1, Time: fixedTime})
//	logger.WithContext(ctx).Fatalf("Level:%s", "fatal")
//}
//
//func TestSkip(t *testing.T) {
//	func(format string, args ...interface{}) {
//		log.WithSkip(1).Infof(format, args...)
//	}("log skip test")
//}
//
//type myLoggerFactory struct{}
//
//func (f *myLoggerFactory) New(arg map[string]interface{}) (log.Appender, error) {
//	return new(myLogger), nil
//}
//
//type myLogger struct {
//}
//
//func (c *myLogger) Level() log.Level {
//	return log.TraceLevel
//}
//
//func (c *myLogger) Print(msg *log.Message) {
//	defer func() { msg.Reuse() }()
//	strLevel := strings.ToUpper(msg.Level.String())
//	var buf bytes.Buffer
//	for _, a := range msg.Args {
//		buf.WriteString(cast.ToString(a))
//	}
//	strTime := msg.Time.Format("2006-01-02T15:04:05.000")
//	fileLine := util.Contract(fmt.Sprintf("%s:%d", msg.File, msg.Line), 48)
//	_, _ = fmt.Printf("[%s][%s][%s] %s\n", strLevel, strTime, fileLine, buf.String())
//}
//
//func TestLogger(t *testing.T) {
//
//	{
//		rootLogger := log.GetLogger("*")
//		rootLogger.Info("this is a info msg")
//		rootLogger.Infof("this is a infof msg")
//
//		tagLogger := rootLogger.WithTag("__tag")
//		tagLogger.Info("this is a info tag msg")
//		tagLogger.Infof("this is a infof tag msg")
//
//		ctx := context.Background()
//		ctx = context.WithValue(ctx, "a", "b")
//		ctxLogger := rootLogger.WithContext(ctx)
//		ctxLogger.Info("this is a info ctx msg")
//		ctxLogger.Infof("this is a infof ctx msg")
//	}
//
//	log.RegisterAppenderFactory("*", new(myLoggerFactory))
//	log.Load("")
//
//	{
//		rootLogger := log.GetLogger("*")
//		rootLogger.Info("this is a info msg")
//		rootLogger.Infof("this is a infof msg")
//
//		tagLogger := rootLogger.WithTag("__tag")
//		tagLogger.Info("this is a info tag msg")
//		tagLogger.Infof("this is a infof tag msg")
//
//		ctx := context.Background()
//		ctx = context.WithValue(ctx, "a", "b")
//		ctxLogger := rootLogger.WithContext(ctx)
//		ctxLogger.Info("this is a info ctx msg")
//		ctxLogger.Infof("this is a infof ctx msg")
//	}
//}
