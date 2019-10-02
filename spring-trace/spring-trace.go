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
	"os"
	"fmt"
	"context"
	"github.com/didi/go-spring/spring-logger"
)

//
// 带 Trace 功能的 Context 对象
//
type TraceContext interface {
	SpringLogger.StdLogger

	// 获取标准的 Context 对象
	Context() context.Context

	// kvs 实际上是 map[string]string 展开
	Logger(kvs ... string) SpringLogger.StdLogger
}

//
// 将日志输出到控制台
//
var console Console

//
// 默认的 TraceContext 版本
//
type DefaultTraceContext struct {
	ctx context.Context

	// 获取一个标准的 Logger 接口, kvs 实际上是 map[string]string 展开
	logger func(ctx context.Context, kvs ... string) SpringLogger.StdLogger
}

//
// 工厂函数
//
func NewDefaultTraceContext(ctx context.Context, logger func(ctx context.Context, kvs ... string) SpringLogger.StdLogger) *DefaultTraceContext {
	return &DefaultTraceContext{
		ctx:    ctx,
		logger: logger,
	}
}

func (c *DefaultTraceContext) Context() context.Context {
	return c.ctx
}

func (c *DefaultTraceContext) Logger(kvs ... string) SpringLogger.StdLogger {
	if c.logger != nil {
		return c.logger(c.ctx, kvs...)
	} else {
		return &console
	}
}

func (c *DefaultTraceContext) Debugf(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger(c.ctx).Debugf(format, args...)
	} else {
		console.Debugf(format, args...)
	}
}

func (c *DefaultTraceContext) Debug(args ...interface{}) {
	if c.logger != nil {
		c.logger(c.ctx).Debug(args...)
	} else {
		console.Debug(args...)
	}
}

func (c *DefaultTraceContext) Infof(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger(c.ctx).Infof(format, args...)
	} else {
		console.Infof(format, args...)
	}
}

func (c *DefaultTraceContext) Info(args ...interface{}) {
	if c.logger != nil {
		c.logger(c.ctx).Info(args...)
	} else {
		console.Info(args...)
	}
}

func (c *DefaultTraceContext) Warnf(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger(c.ctx).Warnf(format, args...)
	} else {
		console.Warnf(format, args...)
	}
}

func (c *DefaultTraceContext) Warn(args ...interface{}) {
	if c.logger != nil {
		c.logger(c.ctx).Warn(args...)
	} else {
		console.Warn(args...)
	}
}

func (c *DefaultTraceContext) Errorf(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger(c.ctx).Errorf(format, args...)
	} else {
		console.Errorf(format, args...)
	}
}

func (c *DefaultTraceContext) Error(args ...interface{}) {
	if c.logger != nil {
		c.logger(c.ctx).Error(args...)
	} else {
		console.Error(args...)
	}
}

func (c *DefaultTraceContext) Fatalf(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger(c.ctx).Fatalf(format, args...)
	} else {
		console.Fatalf(format, args...)
		os.Exit(0)
	}
}

func (c *DefaultTraceContext) Fatal(args ...interface{}) {
	if c.logger != nil {
		c.logger(c.ctx).Fatal(args...)
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
