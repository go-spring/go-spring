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
	"strings"
	"testing"

	"github.com/go-spring/go-spring/spring-logger"
)

type ContextLogger struct {
	ctx  context.Context
	tags []string
}

func (l *ContextLogger) CtxString() string {
	if v := l.ctx.Value("trace_id"); v != nil {
		return "trace_id:" + v.(string)
	}
	return ""
}

func (l *ContextLogger) TagString() string {
	return strings.Join(l.tags, ",")
}

func (l *ContextLogger) Debugf(format string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+l.CtxString()+" "+l.TagString()+format+"\n", args...)
}

func (l *ContextLogger) Debug(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Infof(format string, args ...interface{}) {
	fmt.Printf("[INFO] "+l.CtxString()+" "+l.TagString()+format+"\n", args...)
}

func (l *ContextLogger) Info(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Warnf(format string, args ...interface{}) {
	fmt.Printf("[WARN] "+l.CtxString()+" "+l.TagString()+format+"\n", args...)
}

func (l *ContextLogger) Warn(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Errorf(format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+l.CtxString()+" "+l.TagString()+format+"\n", args...)
}

func (l *ContextLogger) Error(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Fatalf(format string, args ...interface{}) {
	fmt.Printf("[FATAL] "+l.CtxString()+" "+l.TagString()+format+"\n", args...)
	os.Exit(0)
}

func (l *ContextLogger) Fatal(args ...interface{}) {
	fmt.Println(args...)
	os.Exit(0)
}

func NewContextLogger(ctx context.Context, tags ...string) SpringLogger.StdLogger {
	return &ContextLogger{
		ctx:  ctx,
		tags: tags,
	}
}

func TestDefaultTraceContext(t *testing.T) {

	// 设置全局转换函数
	Logger = NewContextLogger

	tracer := &DefaultTraceContext{}

	tracer.ContextFunc = func() context.Context {
		return context.WithValue(nil, "trace_id", "0689")
	}

	fmt.Println()

	tracer.LogDebugf("level:%s", "debug")
	tracer.LogInfof("level:%s", "info")
	tracer.LogWarnf("level:%s", "warn")
	tracer.LogErrorf("level:%s", "error")
	//tracer.LogFatalf("level:%s", "fatal")

	fmt.Println()

	tracer.Logger("__in").Debugf("level:%s", "debug")
	tracer.Logger("__in").Infof("level:%s", "info")
	tracer.Logger("__in").Warnf("level:%s", "warn")
	tracer.Logger("__in").Errorf("level:%s", "error")
	//tracer.Logger("__in").Fatalf("level:%s", "fatal")

	fmt.Println()
}
