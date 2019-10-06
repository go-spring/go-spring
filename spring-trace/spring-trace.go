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

package SpringTrace

import (
	"context"
	"fmt"
	"os"

	"github.com/didi/go-spring/spring-logger"
)

//
// 带 Trace 功能的 Context 对象
//
type TraceContext interface {
	SpringLogger.PrefixLogger

	// 获取标准的 Context 对象
	Context() context.Context

	// 获取一个标准的 Logger 接口
	Logger(tags ...string) SpringLogger.StdLogger
}

//
// 将日志输出到控制台
//
var console Console

//
// 默认的 TraceContext 版本
//
type DefaultTraceContext struct {
	ContextFunc func() context.Context
}

//
// 获取一个标准的 Logger 接口
//
var Logger func(ctx context.Context, tags ...string) SpringLogger.StdLogger

func (c *DefaultTraceContext) Context() context.Context {
	if c.ContextFunc != nil {
		return c.ContextFunc()
	}
	return context.TODO()
}

func (c *DefaultTraceContext) Logger(tags ...string) SpringLogger.StdLogger {
	if Logger != nil {
		return Logger(c.Context(), tags...)
	} else {
		return &console
	}
}

func (c *DefaultTraceContext) LogDebugf(format string, args ...interface{}) {
	if Logger != nil {
		Logger(c.Context()).Debugf(format, args...)
	} else {
		console.Debugf(format, args...)
	}
}

func (c *DefaultTraceContext) LogDebug(args ...interface{}) {
	if Logger != nil {
		Logger(c.Context()).Debug(args...)
	} else {
		console.Debug(args...)
	}
}

func (c *DefaultTraceContext) LogInfof(format string, args ...interface{}) {
	if Logger != nil {
		Logger(c.Context()).Infof(format, args...)
	} else {
		console.Infof(format, args...)
	}
}

func (c *DefaultTraceContext) LogInfo(args ...interface{}) {
	if Logger != nil {
		Logger(c.Context()).Info(args...)
	} else {
		console.Info(args...)
	}
}

func (c *DefaultTraceContext) LogWarnf(format string, args ...interface{}) {
	if Logger != nil {
		Logger(c.Context()).Warnf(format, args...)
	} else {
		console.Warnf(format, args...)
	}
}

func (c *DefaultTraceContext) LogWarn(args ...interface{}) {
	if Logger != nil {
		Logger(c.Context()).Warn(args...)
	} else {
		console.Warn(args...)
	}
}

func (c *DefaultTraceContext) LogErrorf(format string, args ...interface{}) {
	if Logger != nil {
		Logger(c.Context()).Errorf(format, args...)
	} else {
		console.Errorf(format, args...)
	}
}

func (c *DefaultTraceContext) LogError(args ...interface{}) {
	if Logger != nil {
		Logger(c.Context()).Error(args...)
	} else {
		console.Error(args...)
	}
}

func (c *DefaultTraceContext) LogFatalf(format string, args ...interface{}) {
	if Logger != nil {
		Logger(c.Context()).Fatalf(format, args...)
	} else {
		console.Fatalf(format, args...)
		os.Exit(0)
	}
}

func (c *DefaultTraceContext) LogFatal(args ...interface{}) {
	if Logger != nil {
		Logger(c.Context()).Fatal(args...)
	} else {
		console.Fatal(args...)
		os.Exit(0)
	}
}

//
// 控制台打印
//
type Console struct {
}

func (c *Console) Debugf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (c *Console) Debug(args ...interface{}) {
	fmt.Println(args...)
}

func (c *Console) Infof(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (c *Console) Info(args ...interface{}) {
	fmt.Println(args...)
}

func (c *Console) Warnf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (c *Console) Warn(args ...interface{}) {
	fmt.Println(args...)
}

func (c *Console) Errorf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (c *Console) Error(args ...interface{}) {
	fmt.Println(args...)
}

func (c *Console) Fatalf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
	os.Exit(0)
}

func (c *Console) Fatal(args ...interface{}) {
	fmt.Println(args...)
	os.Exit(0)
}
