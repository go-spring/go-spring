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
	"runtime"
	"strings"
	"testing"

	"github.com/go-spring/spring-core/log"
)

func TestDefault(t *testing.T) {

	old := log.SetLevel(log.TraceLevel)
	defer log.SetLevel(old)

	log.Trace("a", "=", "1")
	log.Tracef("a=%d", 1)

	log.Debug("a", "=", "1")
	log.Debugf("a=%d", 1)

	log.Info("a", "=", "1")
	log.Infof("a=%d", 1)

	log.Warn("a", "=", "1")
	log.Warnf("a=%d", 1)

	log.Error("a", "=", "1")
	log.Errorf("a=%d", 1)

	t.Run("panic#00", func(t *testing.T) {
		defer func() { fmt.Println(recover()) }()
		log.Panic("error")
	})

	t.Run("panic#01", func(t *testing.T) {
		defer func() { fmt.Println(recover()) }()
		log.Panic(errors.New("error"))
	})

	t.Run("panic#02", func(t *testing.T) {
		defer func() { fmt.Println(recover()) }()
		log.Panicf("error: %d", 404)
	})

	// log.Fatal("a", "=", "1")
	// log.Fatalf("a=%d", 1)
}

func myOutput(skip int, level log.Level, e *log.Entry) {

	tag := e.GetTag()
	if len(tag) > 0 {
		tag += " "
	}

	ctxString := func(ctx context.Context) string {
		if ctx != nil {
			if v := ctx.Value("trace_id"); v != nil {
				return "trace_id:" + v.(string)
			}
		}
		return ""
	}

	_, file, line, _ := runtime.Caller(skip + 1)
	fmt.Printf("[%s] %s:%d %s %s%s\n", strings.ToUpper(level.String()), file, line, ctxString(e.GetCtx()), tag, e.GetMsg())
}

func TestEntry(t *testing.T) {
	ctx := context.WithValue(context.TODO(), "trace_id", "0689")

	old := log.SetOutput(myOutput)
	defer log.SetOutput(old)

	logger := log.Ctx(ctx)
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

	logger = logger.Tag("__in")
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

	logger = log.Tag("__in")
	logger.Ctx(ctx).Trace("level:", "trace")
	logger.Ctx(ctx).Tracef("level:%s", "trace")
	logger.Ctx(ctx).Debug("level:", "debug")
	logger.Ctx(ctx).Debugf("level:%s", "debug")
	logger.Ctx(ctx).Info("level:", "info")
	logger.Ctx(ctx).Infof("level:%s", "info")
	logger.Ctx(ctx).Warn("level:", "warn")
	logger.Ctx(ctx).Warnf("level:%s", "warn")
	logger.Ctx(ctx).Error("level:", "error")
	logger.Ctx(ctx).Errorf("level:%s", "error")
	logger.Ctx(ctx).Panic("level:", "panic")
	logger.Ctx(ctx).Panicf("level:%s", "panic")
	logger.Ctx(ctx).Fatal("level:", "fatal")
	logger.Ctx(ctx).Fatalf("level:%s", "fatal")
}
