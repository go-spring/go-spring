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

package SpringLogger_test

import (
	"context"

	"github.com/go-spring/spring-logger"
)

func Example_contextLogger() {
	// 设置全局转换函数。
	SpringLogger.Logger = func(ctx context.Context, tags ...string) SpringLogger.StdLogger {
		return &ContextLogger{ctx: ctx, tags: tags}
	}

	ctx := context.WithValue(context.TODO(), "trace_id", "0689")
	tracer := SpringLogger.NewDefaultContextLogger(ctx)

	tracer.LogTrace("level:", "trace")
	tracer.LogTracef("level:%s", "trace")
	tracer.LogDebug("level:", "debug")
	tracer.LogDebugf("level:%s", "debug")
	tracer.LogInfo("level:", "info")
	tracer.LogInfof("level:%s", "info")
	tracer.LogWarn("level:", "warn")
	tracer.LogWarnf("level:%s", "warn")
	tracer.LogError("level:", "error")
	tracer.LogErrorf("level:%s", "error")
	tracer.LogPanic("level:", "panic")
	tracer.LogPanicf("level:%s", "panic")
	tracer.LogFatal("level:", "fatal")
	tracer.LogFatalf("level:%s", "fatal")

	tracer.Logger("__in").Trace("level:", "trace")
	tracer.Logger("__in").Tracef("level:%s", "trace")
	tracer.Logger("__in").Debug("level:", "debug")
	tracer.Logger("__in").Debugf("level:%s", "debug")
	tracer.Logger("__in").Info("level:", "info")
	tracer.Logger("__in").Infof("level:%s", "info")
	tracer.Logger("__in").Warn("level:", "warn")
	tracer.Logger("__in").Warnf("level:%s", "warn")
	tracer.Logger("__in").Error("level:", "error")
	tracer.Logger("__in").Errorf("level:%s", "error")
	tracer.Logger("__in").Panic("level:", "panic")
	tracer.Logger("__in").Panicf("level:%s", "panic")
	tracer.Logger("__in").Fatal("level:", "fatal")
	tracer.Logger("__in").Fatalf("level:%s", "fatal")
}
