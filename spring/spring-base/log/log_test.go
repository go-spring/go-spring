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
	"github.com/go-spring/spring-base/code"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
)

func TestAtomicAndMutex(t *testing.T) {
	t.SkipNow()

	k := int32(0)
	count := 1000000000

	// 直接读取，10亿次，313.234156ms。
	start := time.Now()
	for i := 0; i < count; i++ {
		j := k
		_ = j
	}
	fmt.Println(time.Since(start))

	// 原子读取，10亿次，332.547066ms。
	start = time.Now()
	for i := 0; i < count; i++ {
		j := atomic.LoadInt32(&k)
		_ = j
	}
	k = 0
	fmt.Println(time.Since(start))

	// 原子累加，10亿次，6.251721832s。
	start = time.Now()
	for i := 0; i < count; i++ {
		atomic.AddInt32(&k, 1)
	}
	k = 0
	fmt.Println(time.Since(start))

	// atomic.Value，10亿次，978.367782ms。
	var v atomic.Value
	v.Store(k)
	start = time.Now()
	for i := 0; i < count; i++ {
		j := v.Load().(int32)
		_ = j
	}
	fmt.Println(time.Since(start))

	// 使用读锁，10亿次，12.758831296s。
	var mux sync.RWMutex
	start = time.Now()
	for i := 0; i < count; i++ {
		mux.RLock()
		j := k
		_ = j
		mux.RUnlock()
	}
	fmt.Println(time.Since(start))
}

func TestGetLogger(t *testing.T) {
	logger := log.GetLogger()
	assert.Equal(t, logger.Name(), "github.com/go-spring/spring-base/log_test")
}

func TestRootLogger(t *testing.T) {
	_ = log.GetLogger()

	err := log.Refresh("./testdata/root.xml")
	if err != nil {
		t.Fatal(err)
	}

	//exit := int32(0)
	//go func() {
	//	for {
	//		if atomic.LoadInt32(&exit) > 0 {
	//			break
	//		}
	//		//log.Info()
	//	}
	//}()

	time.Sleep(time.Millisecond)
	fmt.Println("done")
	//atomic.StoreInt32(&exit, 1)
	time.Sleep(time.Millisecond)
}

func TestLogger(t *testing.T) {
	logger := log.NewLogger("", log.TraceLevel)

	msg := func(format string, args ...interface{}) *log.Event {
		return logger.WithSkip(1).Infof(format, args...)
	}("log skip test")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "log skip test")

	msg = logger.Trace("a", "=", "1")
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Tracef("a=%d", 1)
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Trace(func() []interface{} {
		return util.T("a", "=", "1")
	})
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Tracef("a=%d", func() []interface{} {
		return util.T(1)
	})
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Debug("a", "=", "1")
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Debugf("a=%d", 1)
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Debug(func() []interface{} {
		return util.T("a", "=", "1")
	})
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Debugf("a=%d", func() []interface{} {
		return util.T(1)
	})
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Info("a", "=", "1")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Infof("a=%d", 1)
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Info(func() []interface{} {
		return util.T("a", "=", "1")
	})
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Infof("a=%d", func() []interface{} {
		return util.T(1)
	})
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Warn("a", "=", "1")
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Warnf("a=%d", 1)
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Warn(func() []interface{} {
		return util.T("a", "=", "1")
	})
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Warnf("a=%d", func() []interface{} {
		return util.T(1)
	})
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Error("a", "=", "1")
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Errorf("a=%d", 1)
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Error(func() []interface{} {
		return util.T("a", "=", "1")
	})
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Errorf("a=%d", func() []interface{} {
		return util.T(1)
	})
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Panic(errors.New("error"))
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "error")

	msg = logger.Panicf("error:%d", 404)
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "error:404")

	msg = logger.Fatal("a", "=", "1")
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "a=1")

	msg = logger.Fatalf("a=%d", 1)
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Text(), "a=1")
}

func TestEntry(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace", "110110")
	logger := log.NewLogger("", log.TraceLevel)

	const tagIn = "__in"
	ctxLogger := logger.WithContext(ctx)

	msg := ctxLogger.Trace("Level:", "trace")
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:trace")

	msg = ctxLogger.Tracef("Level:%s", "trace")
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:trace")

	msg = ctxLogger.Debug("Level:", "debug")
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:debug")

	msg = ctxLogger.Debugf("Level:%s", "debug")
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:debug")

	msg = ctxLogger.Info("Level:", "info")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:info")

	msg = ctxLogger.Infof("Level:%s", "info")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:info")

	msg = ctxLogger.Warn("Level:", "warn")
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:warn")

	msg = ctxLogger.Warnf("Level:%s", "warn")
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:warn")

	msg = ctxLogger.Error(log.ERROR, "Level:", "error")
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	assert.Equal(t, msg.Text(), "Level:error")

	msg = ctxLogger.Errorf(log.ERROR, "Level:%s", "error")
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	assert.Equal(t, msg.Text(), "Level:error")

	msg = ctxLogger.Panic("Level:", "panic")
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:panic")

	msg = ctxLogger.Panicf("Level:%s", "panic")
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:panic")

	msg = ctxLogger.Fatal("Level:", "fatal")
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:fatal")

	msg = ctxLogger.Fatalf("Level:%s", "fatal")
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:fatal")

	msg = ctxLogger.Trace(func() []interface{} {
		return util.T("Level:", "trace")
	})
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:trace")

	msg = ctxLogger.Tracef("Level:%s", func() []interface{} {
		return util.T("trace")
	})
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:trace")

	msg = ctxLogger.Debug(func() []interface{} {
		return util.T("Level:", "debug")
	})
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:debug")

	msg = ctxLogger.Debugf("Level:%s", func() []interface{} {
		return util.T("debug")
	})
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:debug")

	msg = ctxLogger.Info(func() []interface{} {
		return util.T("Level:", "info")
	})
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:info")

	msg = ctxLogger.Infof("Level:%s", func() []interface{} {
		return util.T("info")
	})
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:info")

	msg = ctxLogger.Warn(func() []interface{} {
		return util.T("Level:", "warn")
	})
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:warn")

	msg = ctxLogger.Warnf("Level:%s", func() []interface{} {
		return util.T("warn")
	})
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:warn")

	msg = ctxLogger.Error(log.ERROR, func() []interface{} {
		return util.T("Level:", "error")
	})
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	assert.Equal(t, msg.Text(), "Level:error")

	msg = ctxLogger.Errorf(log.ERROR, "Level:%s", func() []interface{} {
		return util.T("error")
	})
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	assert.Equal(t, msg.Text(), "Level:error")

	ctxLogger = ctxLogger.WithTag(tagIn)

	msg = ctxLogger.Trace("Level:", "trace")
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:trace")

	msg = ctxLogger.Tracef("Level:%s", "trace")
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:trace")

	msg = ctxLogger.Debug("Level:", "debug")
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:debug")

	msg = ctxLogger.Debugf("Level:%s", "debug")
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:debug")

	msg = ctxLogger.Info("Level:", "info")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:info")

	msg = ctxLogger.Infof("Level:%s", "info")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:info")

	msg = ctxLogger.Warn("Level:", "warn")
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:warn")

	msg = ctxLogger.Warnf("Level:%s", "warn")
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:warn")

	msg = ctxLogger.Error(log.ERROR, "Level:", "error")
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	assert.Equal(t, msg.Text(), "Level:error")

	msg = ctxLogger.Errorf(log.ERROR, "Level:%s", "error")
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	assert.Equal(t, msg.Text(), "Level:error")

	msg = ctxLogger.Panic("Level:", "panic")
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:panic")

	msg = ctxLogger.Panicf("Level:%s", "panic")
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:panic")

	msg = ctxLogger.Fatal("Level:", "fatal")
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:fatal")

	msg = ctxLogger.Fatalf("Level:%s", "fatal")
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:fatal")

	tagLogger := logger.WithTag(tagIn)

	msg = tagLogger.WithContext(ctx).Trace("Level:", "trace")
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:trace")

	msg = tagLogger.WithContext(ctx).Tracef("Level:%s", "trace")
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:trace")

	msg = tagLogger.WithContext(ctx).Debug("Level:", "debug")
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:debug")

	msg = tagLogger.WithContext(ctx).Debugf("Level:%s", "debug")
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:debug")

	msg = tagLogger.WithContext(ctx).Info("Level:", "info")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:info")

	msg = tagLogger.WithContext(ctx).Infof("Level:%s", "info")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:info")

	msg = tagLogger.WithContext(ctx).Warn("Level:", "warn")
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:warn")

	msg = tagLogger.WithContext(ctx).Warnf("Level:%s", "warn")
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:warn")

	msg = tagLogger.WithContext(ctx).Error(log.ERROR, "Level:", "error")
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	assert.Equal(t, msg.Text(), "Level:error")

	msg = tagLogger.WithContext(ctx).Errorf(log.ERROR, "Level:%s", "error")
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	assert.Equal(t, msg.Text(), "Level:error")

	msg = tagLogger.WithContext(ctx).Panic("Level:", "panic")
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:panic")

	msg = tagLogger.WithContext(ctx).Panicf("Level:%s", "panic")
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:panic")

	msg = tagLogger.WithContext(ctx).Fatal("Level:", "fatal")
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:fatal")

	msg = tagLogger.WithContext(ctx).Fatalf("Level:%s", "fatal")
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Text(), "Level:fatal")
}
