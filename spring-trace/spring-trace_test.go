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
	"testing"
	"context"
	"github.com/didi/go-spring/spring-logger"
)

type ContextLogger struct {
	ctx context.Context
	kv  map[string]string
}

func (l *ContextLogger) CtxString() string {
	if v := l.ctx.Value("trace_id"); v != nil {
		return "trace_id:" + v.(string)
	}
	return ""
}

func (l *ContextLogger) KVString() string {
	s := ""
	for k, v := range l.kv {
		s += k + ":" + v + " "
	}
	return s
}

func (l *ContextLogger) Debugf(format string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+l.CtxString()+" "+l.KVString()+format+"\n", args...)
}

func (l *ContextLogger) Debug(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Infof(format string, args ...interface{}) {
	fmt.Printf("[INFO] "+l.CtxString()+" "+l.KVString()+format+"\n", args...)
}

func (l *ContextLogger) Info(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Warnf(format string, args ...interface{}) {
	fmt.Printf("[WARN] "+l.CtxString()+" "+l.KVString()+format+"\n", args...)
}

func (l *ContextLogger) Warn(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Errorf(format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+l.CtxString()+" "+l.KVString()+format+"\n", args...)
}

func (l *ContextLogger) Error(args ...interface{}) {
	fmt.Println(args...)
}

func (l *ContextLogger) Fatalf(format string, args ...interface{}) {
	fmt.Printf("[FATAL] "+l.CtxString()+" "+l.KVString()+format+"\n", args...)
	os.Exit(0)
}

func (l *ContextLogger) Fatal(args ...interface{}) {
	fmt.Println(args...)
	os.Exit(0)
}

func NewContextLogger(ctx context.Context, kvs ...string) SpringLogger.StdLogger {

	kv := make(map[string]string)

	for i := 0; i < len(kvs); i += 2 {
		kv[kvs[i]] = kvs[i+1]
	}

	return &ContextLogger{
		ctx: ctx,
		kv:  kv,
	}
}

func TestDefaultTraceContext(t *testing.T) {

	ctx := context.WithValue(nil, "trace_id", "0689")
	tracer := NewDefaultTraceContext(ctx, NewContextLogger)

	fmt.Println()

	tracer.Debugf("level:%s", "debug")
	tracer.Infof("level:%s", "info")
	tracer.Warnf("level:%s", "warn")
	tracer.Errorf("level:%s", "error")
	//tracer.Fatalf("level:%s", "fatal")

	fmt.Println()

	tracer.Logger("tag", "__in").Debugf("level:%s", "debug")
	tracer.Logger("tag", "__in").Infof("level:%s", "info")
	tracer.Logger("tag", "__in").Warnf("level:%s", "warn")
	tracer.Logger("tag", "__in").Errorf("level:%s", "error")
	//tracer.Logger("tag", "__in").Fatalf("level:%s", "fatal")

	fmt.Println()
}
