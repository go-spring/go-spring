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

func Example_stdLogger() {
	SpringLogger.SetLogger(SpringLogger.NewConsole(SpringLogger.InfoLevel))

	SpringLogger.Trace("a", "=", "1")
	SpringLogger.Tracef("a=%d", 1)

	SpringLogger.Debug("a", "=", "1")
	SpringLogger.Debugf("a=%d", 1)

	SpringLogger.Info("a", "=", "1")
	SpringLogger.Infof("a=%d", 1)

	SpringLogger.Warn("a", "=", "1")
	SpringLogger.Warnf("a=%d", 1)

	SpringLogger.Error("a", "=", "1")
	SpringLogger.Errorf("a=%d", 1)
}

func Example_contextLogger() {
	SpringLogger.RegisterContextOutput(&NativeLogger{})
	ctx := context.WithValue(context.TODO(), "trace_id", "0689")

	logger := SpringLogger.WithContext(ctx)
	logger.Trace("level:", "trace")
	logger.Tracef("level:%s", "trace")
	logger.Debug("level:", "debug")
	logger.Debugf("level:%s", "debug")
	logger.Info("level:", "info")
	logger.Infof("level:%s", "info")
	logger.Warn("level:", "warn")
	logger.Warnf("level:%s", "warn")
	logger.Error("level:", "error")
	logger.Errorf("level:%s", "error")
	logger.Panic("level:", "panic")
	logger.Panicf("level:%s", "panic")
	logger.Fatal("level:", "fatal")
	logger.Fatalf("level:%s", "fatal")

	logger.WithTag("__in").Trace("level:", "trace")
	logger.WithTag("__in").Tracef("level:%s", "trace")
	logger.WithTag("__in").Debug("level:", "debug")
	logger.WithTag("__in").Debugf("level:%s", "debug")
	logger.WithTag("__in").Info("level:", "info")
	logger.WithTag("__in").Infof("level:%s", "info")
	logger.WithTag("__in").Warn("level:", "warn")
	logger.WithTag("__in").Warnf("level:%s", "warn")
	logger.WithTag("__in").Error("level:", "error")
	logger.WithTag("__in").Errorf("level:%s", "error")
	logger.WithTag("__in").Panic("level:", "panic")
	logger.WithTag("__in").Panicf("level:%s", "panic")
	logger.WithTag("__in").Fatal("level:", "fatal")
	logger.WithTag("__in").Fatalf("level:%s", "fatal")
}
