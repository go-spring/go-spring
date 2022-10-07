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

type Student struct{}
type student struct{}

func TestGetLogger(t *testing.T) {

	config := `
		<?xml version="1.0" encoding="UTF-8"?>
		<Configuration>
			<Appenders>
				<Console name="Console"/>
			</Appenders>
			<Loggers>
				<Logger name="spring/spring-base/log_test" level="debug">
					<AppenderRef ref="Console">
						<Filters>
							<LevelFilter level="info"/>
						</Filters>
					</AppenderRef>
				</Logger>
				<Root level="debug">
					<AppenderRef ref="Console"/>
					<LevelFilter level="info"/>
				</Root>
			</Loggers>
		</Configuration>
	`

	err := log.RefreshBuffer(config, ".xml")
	if err != nil {
		t.Fatal(err)
	}

	type Class struct{}
	type class struct{}

	logger := log.GetLogger(util.TypeName(new(Student)))
	assert.Equal(t, logger.Name(), "github.com/go-spring/spring-base/log/log_test.Student")
	logger.Info("1")

	logger = log.GetLogger(util.TypeName(new(student)))
	assert.Equal(t, logger.Name(), "github.com/go-spring/spring-base/log/log_test.student")
	logger.Info("2")

	logger = log.GetLogger(util.TypeName(new(Class)))
	assert.Equal(t, logger.Name(), "github.com/go-spring/spring-base/log/log_test.Class")
	logger.Info("3")

	logger = log.GetLogger(util.TypeName(new(class)))
	assert.Equal(t, logger.Name(), "github.com/go-spring/spring-base/log/log_test.class")
	logger.Info("4")

	logger = nil
	assert.Equal(t, util.TypeName(logger), "github.com/go-spring/spring-base/log/log.Logger")
}

func TestLogger(t *testing.T) {

	config := `
		<?xml version="1.0" encoding="UTF-8"?>
		<Configuration>
			<Appenders>
				<Console name="Console"/>
			</Appenders>
			<Loggers>
				<Root level="trace">
					<AppenderRef ref="Console"/>
				</Root>
			</Loggers>
		</Configuration>
	`

	err := log.RefreshBuffer(config, ".xml")
	if err != nil {
		t.Fatal(err)
	}

	logger := log.GetLogger("xxx", log.TraceLevel)

	msg := func(format string, args ...interface{}) *log.Event {
		return logger.WithSkip(1).Infof(format, args...)
	}("log skip test")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "log skip test")

	msg = logger.Trace("a", "=", "1")
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Tracef("a=%d", 1)
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Trace(log.T(func() []interface{} {
		return []interface{}{"a", "=", "1"}
	}))
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Tracef("a=%d", log.T(func() []interface{} {
		return []interface{}{1}
	}))
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Tracew(log.String("a", "1"))
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Tracew(log.W(func() []log.Field {
		return []log.Field{
			log.String("a", "1"),
		}
	}))
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-7)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Debug("a", "=", "1")
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Debugf("a=%d", 1)
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Debug(log.T(func() []interface{} {
		return []interface{}{"a", "=", "1"}
	}))
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Debugf("a=%d", log.T(func() []interface{} {
		return []interface{}{1}
	}))
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Info("a", "=", "1")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Infof("a=%d", 1)
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Info(log.T(func() []interface{} {
		return []interface{}{"a", "=", "1"}
	}))
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Infof("a=%d", log.T(func() []interface{} {
		return []interface{}{1}
	}))
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Warn("a", "=", "1")
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Warnf("a=%d", 1)
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Warn(log.T(func() []interface{} {
		return []interface{}{"a", "=", "1"}
	}))
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Warnf("a=%d", log.T(func() []interface{} {
		return []interface{}{1}
	}))
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Error("a", "=", "1")
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Errorf("a=%d", 1)
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Error(log.T(func() []interface{} {
		return []interface{}{"a", "=", "1"}
	}))
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Errorf("a=%d", log.T(func() []interface{} {
		return []interface{}{1}
	}))
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Panic(errors.New("error"))
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "error")

	msg = logger.Panicf("error:%d", 404)
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "error:404")

	msg = logger.Fatal("a", "=", "1")
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "a=1")

	msg = logger.Fatalf("a=%d", 1)
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	//assert.Equal(t, msg.Msg().Text(), "a=1")
}

func TestEntry(t *testing.T) {

	config := `
		<?xml version="1.0" encoding="UTF-8"?>
		<Configuration>
			<Appenders>
				<Console name="Console"/>
			</Appenders>
			<Loggers>
				<Root level="trace">
					<AppenderRef ref="Console"/>
				</Root>
			</Loggers>
		</Configuration>
	`

	err := log.RefreshBuffer(config, ".xml")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.WithValue(context.Background(), "trace", "110110")
	logger := log.GetLogger("xxx", log.TraceLevel)

	const tagIn = "__in"
	ctxLogger := logger.WithContext(ctx)

	msg := ctxLogger.Trace("Level:", "trace")
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:trace")

	msg = ctxLogger.Tracef("Level:%s", "trace")
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:trace")

	msg = ctxLogger.Debug("Level:", "debug")
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:debug")

	msg = ctxLogger.Debugf("Level:%s", "debug")
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:debug")

	msg = ctxLogger.Info("Level:", "info")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:info")

	msg = ctxLogger.Infof("Level:%s", "info")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:info")

	msg = ctxLogger.Warn("Level:", "warn")
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:warn")

	msg = ctxLogger.Warnf("Level:%s", "warn")
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:warn")

	msg = ctxLogger.Error(log.ERROR, "Level:", "error")
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	//assert.Equal(t, msg.Msg().Text(), "Level:error")

	msg = ctxLogger.Errorf(log.ERROR, "Level:%s", "error")
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	//assert.Equal(t, msg.Msg().Text(), "Level:error")

	msg = ctxLogger.Panic("Level:", "panic")
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:panic")

	msg = ctxLogger.Panicf("Level:%s", "panic")
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:panic")

	msg = ctxLogger.Fatal("Level:", "fatal")
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:fatal")

	msg = ctxLogger.Fatalf("Level:%s", "fatal")
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:fatal")

	msg = ctxLogger.Trace(log.T(func() []interface{} {
		return []interface{}{"Level:", "trace"}
	}))
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:trace")

	msg = ctxLogger.Tracef("Level:%s", log.T(func() []interface{} {
		return []interface{}{"trace"}
	}))
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:trace")

	msg = ctxLogger.Debug(log.T(func() []interface{} {
		return []interface{}{"Level:", "debug"}
	}))
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:debug")

	msg = ctxLogger.Debugf("Level:%s", log.T(func() []interface{} {
		return []interface{}{"debug"}
	}))
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:debug")

	msg = ctxLogger.Info(log.T(func() []interface{} {
		return []interface{}{"Level:", "info"}
	}))
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:info")

	msg = ctxLogger.Infof("Level:%s", log.T(func() []interface{} {
		return []interface{}{"info"}
	}))
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:info")

	msg = ctxLogger.Warn(log.T(func() []interface{} {
		return []interface{}{"Level:", "warn"}
	}))
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:warn")

	msg = ctxLogger.Warnf("Level:%s", log.T(func() []interface{} {
		return []interface{}{"warn"}
	}))
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:warn")

	msg = ctxLogger.Error(log.ERROR, log.T(func() []interface{} {
		return []interface{}{"Level:", "error"}
	}))
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	//assert.Equal(t, msg.Msg().Text(), "Level:error")

	msg = ctxLogger.Errorf(log.ERROR, "Level:%s", log.T(func() []interface{} {
		return []interface{}{"error"}
	}))
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-5)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	//assert.Equal(t, msg.Msg().Text(), "Level:error")

	ctxLogger = ctxLogger.WithTag(tagIn)

	msg = ctxLogger.Trace("Level:", "trace")
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:trace")

	msg = ctxLogger.Tracef("Level:%s", "trace")
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:trace")

	msg = ctxLogger.Debug("Level:", "debug")
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:debug")

	msg = ctxLogger.Debugf("Level:%s", "debug")
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:debug")

	msg = ctxLogger.Info("Level:", "info")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:info")

	msg = ctxLogger.Infof("Level:%s", "info")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:info")

	msg = ctxLogger.Warn("Level:", "warn")
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:warn")

	msg = ctxLogger.Warnf("Level:%s", "warn")
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:warn")

	msg = ctxLogger.Error(log.ERROR, "Level:", "error")
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	//assert.Equal(t, msg.Msg().Text(), "Level:error")

	msg = ctxLogger.Errorf(log.ERROR, "Level:%s", "error")
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	//assert.Equal(t, msg.Msg().Text(), "Level:error")

	msg = ctxLogger.Panic("Level:", "panic")
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:panic")

	msg = ctxLogger.Panicf("Level:%s", "panic")
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:panic")

	msg = ctxLogger.Fatal("Level:", "fatal")
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:fatal")

	msg = ctxLogger.Fatalf("Level:%s", "fatal")
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:fatal")

	tagLogger := logger.WithTag(tagIn)

	msg = tagLogger.WithContext(ctx).Trace("Level:", "trace")
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:trace")

	msg = tagLogger.WithContext(ctx).Tracef("Level:%s", "trace")
	assert.Equal(t, msg.Level(), log.TraceLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:trace")

	msg = tagLogger.WithContext(ctx).Debug("Level:", "debug")
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:debug")

	msg = tagLogger.WithContext(ctx).Debugf("Level:%s", "debug")
	assert.Equal(t, msg.Level(), log.DebugLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:debug")

	msg = tagLogger.WithContext(ctx).Info("Level:", "info")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:info")

	msg = tagLogger.WithContext(ctx).Infof("Level:%s", "info")
	assert.Equal(t, msg.Level(), log.InfoLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:info")

	msg = tagLogger.WithContext(ctx).Warn("Level:", "warn")
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:warn")

	msg = tagLogger.WithContext(ctx).Warnf("Level:%s", "warn")
	assert.Equal(t, msg.Level(), log.WarnLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:warn")

	msg = tagLogger.WithContext(ctx).Error(log.ERROR, "Level:", "error")
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	//assert.Equal(t, msg.Msg().Text(), "Level:error")

	msg = tagLogger.WithContext(ctx).Errorf(log.ERROR, "Level:%s", "error")
	assert.Equal(t, msg.Level(), log.ErrorLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	assert.Equal(t, msg.Entry().Errno(), log.ERROR)
	//assert.Equal(t, msg.Msg().Text(), "Level:error")

	msg = tagLogger.WithContext(ctx).Panic("Level:", "panic")
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:panic")

	msg = tagLogger.WithContext(ctx).Panicf("Level:%s", "panic")
	assert.Equal(t, msg.Level(), log.PanicLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:panic")

	msg = tagLogger.WithContext(ctx).Fatal("Level:", "fatal")
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:fatal")

	msg = tagLogger.WithContext(ctx).Fatalf("Level:%s", "fatal")
	assert.Equal(t, msg.Level(), log.FatalLevel)
	assert.Equal(t, msg.File(), code.File())
	assert.Equal(t, msg.Line(), code.Line()-3)
	assert.Equal(t, msg.Entry().Tag(), tagIn)
	assert.Equal(t, msg.Entry().Context(), ctx)
	//assert.Equal(t, msg.Msg().Text(), "Level:fatal")
}
