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
	SpringLogger.SimpleLogger
}

//
// 默认的 TraceContext 版本
//
type DefaultTraceContext struct {
	ctx    context.Context
	logger SpringLogger.ContextLogger
}

//
// 工厂函数
//
func NewDefaultTraceContext(ctx context.Context, logger SpringLogger.ContextLogger) *DefaultTraceContext {
	return &DefaultTraceContext{
		ctx:    ctx,
		logger: logger,
	}
}

func (c *DefaultTraceContext) Debugf(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger.Debugf(c.ctx, format+"\n", args...)
	} else {
		fmt.Printf(format+"\n", args...)
	}
}

func (c *DefaultTraceContext) Debug(args ...interface{}) {
	if c.logger != nil {
		c.logger.Debug(c.ctx, args...)
	} else {
		fmt.Println(args...)
	}
}

func (c *DefaultTraceContext) Infof(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger.Infof(c.ctx, format+"\n", args...)
	} else {
		fmt.Printf(format+"\n", args...)
	}
}

func (c *DefaultTraceContext) Info(args ...interface{}) {
	if c.logger != nil {
		c.logger.Info(c.ctx, args...)
	} else {
		fmt.Println(args...)
	}
}

func (c *DefaultTraceContext) Warnf(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger.Warnf(c.ctx, format+"\n", args...)
	} else {
		fmt.Printf(format+"\n", args...)
	}
}

func (c *DefaultTraceContext) Warn(args ...interface{}) {
	if c.logger != nil {
		c.logger.Warn(c.ctx, args...)
	} else {
		fmt.Println(args...)
	}
}

func (c *DefaultTraceContext) Errorf(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger.Errorf(c.ctx, format+"\n", args...)
	} else {
		fmt.Printf(format+"\n", args...)
	}
}

func (c *DefaultTraceContext) Error(args ...interface{}) {
	if c.logger != nil {
		c.logger.Error(c.ctx, args...)
	} else {
		fmt.Println(args...)
	}
}

func (c *DefaultTraceContext) Fatalf(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger.Fatalf(c.ctx, format+"\n", args...)
	} else {
		fmt.Printf(format+"\n", args...)
		os.Exit(0)
	}
}

func (c *DefaultTraceContext) Fatal(args ...interface{}) {
	if c.logger != nil {
		c.logger.Fatal(c.ctx, args...)
	} else {
		fmt.Println(args...)
		os.Exit(0)
	}
}
