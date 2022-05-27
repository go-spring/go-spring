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
	t.Skip()

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

	err := log.RefreshXML(`
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

	exit := int32(0)
	go func() {
		for {
			if atomic.LoadInt32(&exit) > 0 {
				break
			}
			log.Info()
		}
	}()

	time.Sleep(time.Millisecond)
	fmt.Println("done")
	atomic.StoreInt32(&exit, 1)
	time.Sleep(time.Millisecond)
}

type mockAppender struct {
	msg *log.Message
}

func (appender *mockAppender) Last() *log.Message {
	return appender.msg
}

func (appender *mockAppender) Append(msg *log.Message) {
	appender.msg = msg
}

func TestLogger(t *testing.T) {

	appender := &mockAppender{}
	logger := log.NewLogger("l", &log.LoggerConfig{
		Level:     log.TraceLevel,
		Appenders: []log.Appender{appender},
	})

	func(format string, args ...interface{}) {
		logger.WithSkip(1).Infof(format, args...)
	}("log skip test")
	assert.Equal(t, appender.Last().Level(), log.InfoLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "log skip test")

	logger.Trace("a", "=", "1")
	assert.Equal(t, appender.Last().Level(), log.TraceLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Tracef("a=%d", 1)
	assert.Equal(t, appender.Last().Level(), log.TraceLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Trace(func() []interface{} {
		return util.T("a", "=", "1")
	})
	assert.Equal(t, appender.Last().Level(), log.TraceLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Tracef("a=%d", func() []interface{} {
		return util.T(1)
	})
	assert.Equal(t, appender.Last().Level(), log.TraceLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Debug("a", "=", "1")
	assert.Equal(t, appender.Last().Level(), log.DebugLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Debugf("a=%d", 1)
	assert.Equal(t, appender.Last().Level(), log.DebugLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Debug(func() []interface{} {
		return util.T("a", "=", "1")
	})
	assert.Equal(t, appender.Last().Level(), log.DebugLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Debugf("a=%d", func() []interface{} {
		return util.T(1)
	})
	assert.Equal(t, appender.Last().Level(), log.DebugLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Info("a", "=", "1")
	assert.Equal(t, appender.Last().Level(), log.InfoLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Infof("a=%d", 1)
	assert.Equal(t, appender.Last().Level(), log.InfoLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Info(func() []interface{} {
		return util.T("a", "=", "1")
	})
	assert.Equal(t, appender.Last().Level(), log.InfoLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Infof("a=%d", func() []interface{} {
		return util.T(1)
	})
	assert.Equal(t, appender.Last().Level(), log.InfoLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Warn("a", "=", "1")
	assert.Equal(t, appender.Last().Level(), log.WarnLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Warnf("a=%d", 1)
	assert.Equal(t, appender.Last().Level(), log.WarnLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Warn(func() []interface{} {
		return util.T("a", "=", "1")
	})
	assert.Equal(t, appender.Last().Level(), log.WarnLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Warnf("a=%d", func() []interface{} {
		return util.T(1)
	})
	assert.Equal(t, appender.Last().Level(), log.WarnLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Error("a", "=", "1")
	assert.Equal(t, appender.Last().Level(), log.ErrorLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Errorf("a=%d", 1)
	assert.Equal(t, appender.Last().Level(), log.ErrorLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Error(func() []interface{} {
		return util.T("a", "=", "1")
	})
	assert.Equal(t, appender.Last().Level(), log.ErrorLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Errorf("a=%d", func() []interface{} {
		return util.T(1)
	})
	assert.Equal(t, appender.Last().Level(), log.ErrorLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Panic(errors.New("error"))
	assert.Equal(t, appender.Last().Level(), log.PanicLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "error")

	logger.Panicf("error:%d", 404)
	assert.Equal(t, appender.Last().Level(), log.PanicLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "error:404")

	logger.Fatal("a", "=", "1")
	assert.Equal(t, appender.Last().Level(), log.FatalLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "a=1")

	logger.Fatalf("a=%d", 1)
	assert.Equal(t, appender.Last().Level(), log.FatalLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Text(), "a=1")
}

func TestEntry(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace", "110110")

	appender := &mockAppender{}
	logger := log.NewLogger("l", &log.LoggerConfig{
		Level:     log.TraceLevel,
		Appenders: []log.Appender{appender},
	})

	const tagIn = "__in"
	ctxLogger := logger.WithContext(ctx)

	ctxLogger.Trace("Level:", "trace")
	assert.Equal(t, appender.Last().Level(), log.TraceLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:trace")

	ctxLogger.Tracef("Level:%s", "trace")
	assert.Equal(t, appender.Last().Level(), log.TraceLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:trace")

	ctxLogger.Debug("Level:", "debug")
	assert.Equal(t, appender.Last().Level(), log.DebugLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:debug")

	ctxLogger.Debugf("Level:%s", "debug")
	assert.Equal(t, appender.Last().Level(), log.DebugLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:debug")

	ctxLogger.Info("Level:", "info")
	assert.Equal(t, appender.Last().Level(), log.InfoLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:info")

	ctxLogger.Infof("Level:%s", "info")
	assert.Equal(t, appender.Last().Level(), log.InfoLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:info")

	ctxLogger.Warn("Level:", "warn")
	assert.Equal(t, appender.Last().Level(), log.WarnLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:warn")

	ctxLogger.Warnf("Level:%s", "warn")
	assert.Equal(t, appender.Last().Level(), log.WarnLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:warn")

	ctxLogger.Error(log.ERROR, "Level:", "error")
	assert.Equal(t, appender.Last().Level(), log.ErrorLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Errno(), log.ERROR)
	assert.Equal(t, appender.Last().Text(), "Level:error")

	ctxLogger.Errorf(log.ERROR, "Level:%s", "error")
	assert.Equal(t, appender.Last().Level(), log.ErrorLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Errno(), log.ERROR)
	assert.Equal(t, appender.Last().Text(), "Level:error")

	ctxLogger.Panic("Level:", "panic")
	assert.Equal(t, appender.Last().Level(), log.PanicLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:panic")

	ctxLogger.Panicf("Level:%s", "panic")
	assert.Equal(t, appender.Last().Level(), log.PanicLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:panic")

	ctxLogger.Fatal("Level:", "fatal")
	assert.Equal(t, appender.Last().Level(), log.FatalLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:fatal")

	ctxLogger.Fatalf("Level:%s", "fatal")
	assert.Equal(t, appender.Last().Level(), log.FatalLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:fatal")

	ctxLogger.Trace(func() []interface{} {
		return util.T("Level:", "trace")
	})
	assert.Equal(t, appender.Last().Level(), log.TraceLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:trace")

	ctxLogger.Tracef("Level:%s", func() []interface{} {
		return util.T("trace")
	})
	assert.Equal(t, appender.Last().Level(), log.TraceLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:trace")

	ctxLogger.Debug(func() []interface{} {
		return util.T("Level:", "debug")
	})
	assert.Equal(t, appender.Last().Level(), log.DebugLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:debug")

	ctxLogger.Debugf("Level:%s", func() []interface{} {
		return util.T("debug")
	})
	assert.Equal(t, appender.Last().Level(), log.DebugLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:debug")

	ctxLogger.Info(func() []interface{} {
		return util.T("Level:", "info")
	})
	assert.Equal(t, appender.Last().Level(), log.InfoLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:info")

	ctxLogger.Infof("Level:%s", func() []interface{} {
		return util.T("info")
	})
	assert.Equal(t, appender.Last().Level(), log.InfoLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:info")

	ctxLogger.Warn(func() []interface{} {
		return util.T("Level:", "warn")
	})
	assert.Equal(t, appender.Last().Level(), log.WarnLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:warn")

	ctxLogger.Warnf("Level:%s", func() []interface{} {
		return util.T("warn")
	})
	assert.Equal(t, appender.Last().Level(), log.WarnLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:warn")

	ctxLogger.Error(log.ERROR, func() []interface{} {
		return util.T("Level:", "error")
	})
	assert.Equal(t, appender.Last().Level(), log.ErrorLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Errno(), log.ERROR)
	assert.Equal(t, appender.Last().Text(), "Level:error")

	ctxLogger.Errorf(log.ERROR, "Level:%s", func() []interface{} {
		return util.T("error")
	})
	assert.Equal(t, appender.Last().Level(), log.ErrorLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-5)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Errno(), log.ERROR)
	assert.Equal(t, appender.Last().Text(), "Level:error")

	ctxLogger = ctxLogger.WithTag(tagIn)

	ctxLogger.Trace("Level:", "trace")
	assert.Equal(t, appender.Last().Level(), log.TraceLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:trace")

	ctxLogger.Tracef("Level:%s", "trace")
	assert.Equal(t, appender.Last().Level(), log.TraceLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:trace")

	ctxLogger.Debug("Level:", "debug")
	assert.Equal(t, appender.Last().Level(), log.DebugLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:debug")

	ctxLogger.Debugf("Level:%s", "debug")
	assert.Equal(t, appender.Last().Level(), log.DebugLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:debug")

	ctxLogger.Info("Level:", "info")
	assert.Equal(t, appender.Last().Level(), log.InfoLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:info")

	ctxLogger.Infof("Level:%s", "info")
	assert.Equal(t, appender.Last().Level(), log.InfoLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:info")

	ctxLogger.Warn("Level:", "warn")
	assert.Equal(t, appender.Last().Level(), log.WarnLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:warn")

	ctxLogger.Warnf("Level:%s", "warn")
	assert.Equal(t, appender.Last().Level(), log.WarnLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:warn")

	ctxLogger.Error(log.ERROR, "Level:", "error")
	assert.Equal(t, appender.Last().Level(), log.ErrorLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Errno(), log.ERROR)
	assert.Equal(t, appender.Last().Text(), "Level:error")

	ctxLogger.Errorf(log.ERROR, "Level:%s", "error")
	assert.Equal(t, appender.Last().Level(), log.ErrorLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Errno(), log.ERROR)
	assert.Equal(t, appender.Last().Text(), "Level:error")

	ctxLogger.Panic("Level:", "panic")
	assert.Equal(t, appender.Last().Level(), log.PanicLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:panic")

	ctxLogger.Panicf("Level:%s", "panic")
	assert.Equal(t, appender.Last().Level(), log.PanicLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:panic")

	ctxLogger.Fatal("Level:", "fatal")
	assert.Equal(t, appender.Last().Level(), log.FatalLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:fatal")

	ctxLogger.Fatalf("Level:%s", "fatal")
	assert.Equal(t, appender.Last().Level(), log.FatalLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:fatal")

	tagLogger := logger.WithTag(tagIn)

	tagLogger.WithContext(ctx).Trace("Level:", "trace")
	assert.Equal(t, appender.Last().Level(), log.TraceLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:trace")

	tagLogger.WithContext(ctx).Tracef("Level:%s", "trace")
	assert.Equal(t, appender.Last().Level(), log.TraceLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:trace")

	tagLogger.WithContext(ctx).Debug("Level:", "debug")
	assert.Equal(t, appender.Last().Level(), log.DebugLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:debug")

	tagLogger.WithContext(ctx).Debugf("Level:%s", "debug")
	assert.Equal(t, appender.Last().Level(), log.DebugLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:debug")

	tagLogger.WithContext(ctx).Info("Level:", "info")
	assert.Equal(t, appender.Last().Level(), log.InfoLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:info")

	tagLogger.WithContext(ctx).Infof("Level:%s", "info")
	assert.Equal(t, appender.Last().Level(), log.InfoLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:info")

	tagLogger.WithContext(ctx).Warn("Level:", "warn")
	assert.Equal(t, appender.Last().Level(), log.WarnLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:warn")

	tagLogger.WithContext(ctx).Warnf("Level:%s", "warn")
	assert.Equal(t, appender.Last().Level(), log.WarnLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:warn")

	tagLogger.WithContext(ctx).Error(log.ERROR, "Level:", "error")
	assert.Equal(t, appender.Last().Level(), log.ErrorLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Errno(), log.ERROR)
	assert.Equal(t, appender.Last().Text(), "Level:error")

	tagLogger.WithContext(ctx).Errorf(log.ERROR, "Level:%s", "error")
	assert.Equal(t, appender.Last().Level(), log.ErrorLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Errno(), log.ERROR)
	assert.Equal(t, appender.Last().Text(), "Level:error")

	tagLogger.WithContext(ctx).Panic("Level:", "panic")
	assert.Equal(t, appender.Last().Level(), log.PanicLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:panic")

	tagLogger.WithContext(ctx).Panicf("Level:%s", "panic")
	assert.Equal(t, appender.Last().Level(), log.PanicLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:panic")

	tagLogger.WithContext(ctx).Fatal("Level:", "fatal")
	assert.Equal(t, appender.Last().Level(), log.FatalLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:fatal")

	tagLogger.WithContext(ctx).Fatalf("Level:%s", "fatal")
	assert.Equal(t, appender.Last().Level(), log.FatalLevel)
	assert.Equal(t, appender.Last().File(), code.File())
	assert.Equal(t, appender.Last().Line(), code.Line()-3)
	assert.Equal(t, appender.Last().Tag(), tagIn)
	assert.Equal(t, appender.Last().Context(), ctx)
	assert.Equal(t, appender.Last().Text(), "Level:fatal")
}
