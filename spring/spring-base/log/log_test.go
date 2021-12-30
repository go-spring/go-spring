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

	{
		o.EXPECT().Do(TraceLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 57,
			time: fixedTime,
		})
		Trace("a", "=", "1")
		o.EXPECT().Do(TraceLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 64,
			time: fixedTime,
		})
		Tracef("a=%d", 1)
	}

	{
		o.EXPECT().Do(TraceLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 74,
			time: fixedTime,
		})
		Trace(func() []interface{} {
			return T("a", "=", "1")
		})
		o.EXPECT().Do(TraceLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 83,
			time: fixedTime,
		})
		Tracef("a=%d", func() []interface{} {
			return T(1)
		})
	}

	{
		o.EXPECT().Do(DebugLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 95,
			time: fixedTime,
		})
		Debug("a", "=", "1")
		o.EXPECT().Do(DebugLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 102,
			time: fixedTime,
		})
		Debugf("a=%d", 1)
	}

	{
		o.EXPECT().Do(DebugLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 112,
			time: fixedTime,
		})
		Debug(func() []interface{} {
			return T("a", "=", "1")
		})
		o.EXPECT().Do(DebugLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 121,
			time: fixedTime,
		})
		Debugf("a=%d", func() []interface{} {
			return T(1)
		})
	}

	{
		o.EXPECT().Do(InfoLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 133,
			time: fixedTime,
		})
		Info("a", "=", "1")
		o.EXPECT().Do(InfoLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 140,
			time: fixedTime,
		})
		Infof("a=%d", 1)
	}

	{
		o.EXPECT().Do(InfoLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 150,
			time: fixedTime,
		})
		Info(func() []interface{} {
			return T("a", "=", "1")
		})
		o.EXPECT().Do(InfoLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 159,
			time: fixedTime,
		})
		Infof("a=%d", func() []interface{} {
			return T(1)
		})
	}

	{
		o.EXPECT().Do(WarnLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 171,
			time: fixedTime,
		})
		Warn("a", "=", "1")
		o.EXPECT().Do(WarnLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 178,
			time: fixedTime,
		})
		Warnf("a=%d", 1)
	}

	{
		o.EXPECT().Do(WarnLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 188,
			time: fixedTime,
		})
		Warn(func() []interface{} {
			return T("a", "=", "1")
		})
		o.EXPECT().Do(WarnLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 197,
			time: fixedTime,
		})
		Warnf("a=%d", func() []interface{} {
			return T(1)
		})
	}

	{
		o.EXPECT().Do(ErrorLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 209,
			time: fixedTime,
		})
		Error("a", "=", "1")
		o.EXPECT().Do(ErrorLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 216,
			time: fixedTime,
		})
		Errorf("a=%d", 1)
	}

	{
		o.EXPECT().Do(ErrorLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 226,
			time: fixedTime,
		})
		Error(func() []interface{} {
			return T("a", "=", "1")
		})
		o.EXPECT().Do(ErrorLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 235,
			time: fixedTime,
		})
		Errorf("a=%d", func() []interface{} {
			return T(1)
		})
	}

	{
		o.EXPECT().Do(PanicLevel, &Message{
			text: "error",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 247,
			time: fixedTime,
		})
		Panic(errors.New("error"))
		o.EXPECT().Do(PanicLevel, &Message{
			text: "error:404",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 254,
			time: fixedTime,
		})
		Panicf("error:%d", 404)
	}

	{
		o.EXPECT().Do(FatalLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 264,
			time: fixedTime,
		})
		Fatal("a", "=", "1")
		o.EXPECT().Do(FatalLevel, &Message{
			text: "a=1",
			file: "/Users/didi/GitHub/go-spring/go-spring/spring/spring-base/log/log_test.go",
			line: 271,
			time: fixedTime,
		})
		Fatalf("a=%d", 1)
	}
}

//
//type traceIDKeyType int
//
//var traceIDKey traceIDKeyType
//
//func myOutput(level Level, e *Entry) {
//
//	msg := e.GetMsg()
//	tag := e.GetTag()
//	if len(tag) > 0 {
//		tag += " "
//	}
//
//	strCtx := func(ctx context.Context) string {
//		if ctx == nil {
//			return ""
//		}
//		v := ctx.Value(traceIDKey)
//		if v == nil {
//			return ""
//		}
//		return "trace_id:" + v.(string)
//	}(e.GetCtx())
//
//	line := e.GetLine()
//	file := e.GetFile()
//	strLevel := strings.ToUpper(level.String())
//	strTime := e.GetTime().Format("2006-01-02 03-04-05.000")
//	fmt.Printf("[%s] %s %s:%d %s %s%s\n", strLevel, strTime, file, line, strCtx, tag, msg)
//}
//
//func TestEntry(t *testing.T) {
//	ctx := context.WithValue(context.TODO(), traceIDKey, "0689")
//
//	SetLevel(TraceLevel)
//	SetOutput(FuncOutput(myOutput))
//	defer Reset()
//
//	logger := Ctx(ctx)
//	logger.Trace("level:", "trace")
//	logger.Tracef("level:%s", "trace")
//	logger.Debug("level:", "debug")
//	logger.Debugf("level:%s", "debug")
//	logger.Info("level:", "info")
//	logger.Infof("level:%s", "info")
//	logger.Warn("level:", "warn")
//	logger.Warnf("level:%s", "warn")
//	logger.Error("level:", "error")
//	logger.Errorf("level:%s", "error")
//	logger.Panic("level:", "panic")
//	logger.Panicf("level:%s", "panic")
//	logger.Fatal("level:", "fatal")
//	logger.Fatalf("level:%s", "fatal")
//
//	logger.Trace(func() []interface{} {
//		return T("level:", "trace")
//	})
//
//	logger.Tracef("level:%s", func() []interface{} {
//		return T("trace")
//	})
//
//	logger.Debug(func() []interface{} {
//		return T("level:", "debug")
//	})
//
//	logger.Debugf("level:%s", func() []interface{} {
//		return T("debug")
//	})
//
//	logger.Info(func() []interface{} {
//		return T("level:", "info")
//	})
//
//	logger.Infof("level:%s", func() []interface{} {
//		return T("info")
//	})
//
//	logger.Warn(func() []interface{} {
//		return T("level:", "warn")
//	})
//
//	logger.Warnf("level:%s", func() []interface{} {
//		return T("warn")
//	})
//
//	logger.Error(func() []interface{} {
//		return T("level:", "error")
//	})
//
//	logger.Errorf("level:%s", func() []interface{} {
//		return T("error")
//	})
//
//	logger = logger.Tag("__in")
//	logger.Trace("level:", "trace")
//	logger.Tracef("level:%s", "trace")
//	logger.Debug("level:", "debug")
//	logger.Debugf("level:%s", "debug")
//	logger.Info("level:", "info")
//	logger.Infof("level:%s", "info")
//	logger.Warn("level:", "warn")
//	logger.Warnf("level:%s", "warn")
//	logger.Error("level:", "error")
//	logger.Errorf("level:%s", "error")
//	logger.Panic("level:", "panic")
//	logger.Panicf("level:%s", "panic")
//	logger.Fatal("level:", "fatal")
//	logger.Fatalf("level:%s", "fatal")
//
//	logger = Tag("__in")
//	logger.Ctx(ctx).Trace("level:", "trace")
//	logger.Ctx(ctx).Tracef("level:%s", "trace")
//	logger.Ctx(ctx).Debug("level:", "debug")
//	logger.Ctx(ctx).Debugf("level:%s", "debug")
//	logger.Ctx(ctx).Info("level:", "info")
//	logger.Ctx(ctx).Infof("level:%s", "info")
//	logger.Ctx(ctx).Warn("level:", "warn")
//	logger.Ctx(ctx).Warnf("level:%s", "warn")
//	logger.Ctx(ctx).Error("level:", "error")
//	logger.Ctx(ctx).Errorf("level:%s", "error")
//	logger.Ctx(ctx).Panic("level:", "panic")
//	logger.Ctx(ctx).Panicf("level:%s", "panic")
//	logger.Ctx(ctx).Fatal("level:", "fatal")
//	logger.Ctx(ctx).Fatalf("level:%s", "fatal")
//}

func TestSkip(t *testing.T) {
	func(format string, args ...interface{}) {
		Skip(1).Infof(format, args...)
	}("log skip test")
}
