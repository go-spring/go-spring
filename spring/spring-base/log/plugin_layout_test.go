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
	"fmt"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/clock"
	"github.com/go-spring/spring-base/code"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/log"
)

func TestParseColorStyle(t *testing.T) {

	v, err := log.ParseColorStyle("abc")
	assert.Error(t, err, "invalid color style 'abc'")

	v, err = log.ParseColorStyle("none")
	assert.Nil(t, err)
	assert.Equal(t, v, log.ColorStyleNone)

	v, err = log.ParseColorStyle("normal")
	assert.Nil(t, err)
	assert.Equal(t, v, log.ColorStyleNormal)

	v, err = log.ParseColorStyle("bright")
	assert.Nil(t, err)
	assert.Equal(t, v, log.ColorStyleBright)
}

func TestPatternLayout(t *testing.T) {

	layout := log.PatternLayout{
		ColorStyle: log.ColorStyleNormal,
		Pattern:    "[:level][:time][:fileline][:msg]",
	}
	err := layout.Init()
	assert.Nil(t, err)

	ctx, _ := knife.New(context.Background())
	ctx = context.WithValue(ctx, "traceKey", "123456789")
	_ = clock.SetFixedTime(ctx, time.Date(2022, 9, 30, 8, 0, 0, 0, time.UTC))

	e := &log.Event{
		//Entry: new(log.ContextEntry).WithTag("tagABC").WithContext(ctx),
		File:  code.File(),
		Line:  code.Line(),
		Level: log.InfoLevel,
		Fields: []log.Field{
			log.String("field_a", "abc"),
			log.Int("field_b", 5),
		},
	}

	e.Level = log.TraceLevel
	b, err := layout.ToBytes(e)
	assert.Nil(t, err)
	fmt.Print(string(b))
	//assert.Equal(t, string(b), "[TRACE][2022-09-30T08:00:00.000][...ring/spring-base/log/plugin_layout_test.go:43] tagABC||field_a=abc||field_b=5\n")

	e.Level = log.DebugLevel
	b, err = layout.ToBytes(e)
	assert.Nil(t, err)
	fmt.Print(string(b))
	//assert.Equal(t, string(b), "[DEBUG][2022-09-30T08:00:00.000][...ring/spring-base/log/plugin_layout_test.go:43] tagABC||field_a=abc||field_b=5\n")

	e.Level = log.InfoLevel
	b, err = layout.ToBytes(e)
	assert.Nil(t, err)
	fmt.Print(string(b))
	//assert.Equal(t, string(b), "[INFO][2022-09-30T08:00:00.000][...ring/spring-base/log/plugin_layout_test.go:43] tagABC||field_a=abc||field_b=5\n")

	e.Level = log.WarnLevel
	b, err = layout.ToBytes(e)
	assert.Nil(t, err)
	fmt.Print(string(b))
	//assert.Equal(t, string(b), "[WARN][2022-09-30T08:00:00.000][...ring/spring-base/log/plugin_layout_test.go:43] tagABC||field_a=abc||field_b=5\n")

	e.Level = log.ErrorLevel
	b, err = layout.ToBytes(e)
	assert.Nil(t, err)
	fmt.Print(string(b))
	//assert.Equal(t, string(b), "[ERROR][2022-09-30T08:00:00.000][...ring/spring-base/log/plugin_layout_test.go:43] tagABC||field_a=abc||field_b=5\n")

	e.Level = log.PanicLevel
	b, err = layout.ToBytes(e)
	assert.Nil(t, err)
	fmt.Print(string(b))
	//assert.Equal(t, string(b), "[PANIC][2022-09-30T08:00:00.000][...ring/spring-base/log/plugin_layout_test.go:43] tagABC||field_a=abc||field_b=5\n")

	e.Level = log.FatalLevel
	b, err = layout.ToBytes(e)
	assert.Nil(t, err)
	fmt.Print(string(b))
	//assert.Equal(t, string(b), "[FATAL][2022-09-30T08:00:00.000][...ring/spring-base/log/plugin_layout_test.go:43] tagABC||field_a=abc||field_b=5\n")
}

func TestJSONLayout(t *testing.T) {

	layout := log.JSONLayout{}

	ctx, _ := knife.New(context.Background())
	ctx = context.WithValue(ctx, "traceKey", "123456789")
	_ = clock.SetFixedTime(ctx, time.Date(2022, 9, 30, 8, 0, 0, 0, time.UTC))

	e := &log.Event{
		//Entry: new(log.ContextEntry).WithTag("tagABC").WithContext(ctx),
		File:  code.File(),
		Line:  code.Line(),
		Level: log.InfoLevel,
		Fields: []log.Field{
			log.String("field_a", "abc"),
			log.Int("field_b", 5),
		},
	}

	e.Level = log.TraceLevel
	b, err := layout.ToBytes(e)
	assert.Nil(t, err)
	fmt.Print(string(b))
	//assert.Equal(t, string(b), "{\"field_a\":\"abc\",\"field_b\":5}\n")

	e.Level = log.DebugLevel
	b, err = layout.ToBytes(e)
	assert.Nil(t, err)
	fmt.Print(string(b))
	//assert.Equal(t, string(b), "{\"field_a\":\"abc\",\"field_b\":5}\n")

	e.Level = log.InfoLevel
	b, err = layout.ToBytes(e)
	assert.Nil(t, err)
	fmt.Print(string(b))
	//assert.Equal(t, string(b), "{\"field_a\":\"abc\",\"field_b\":5}\n")

	e.Level = log.WarnLevel
	b, err = layout.ToBytes(e)
	assert.Nil(t, err)
	fmt.Print(string(b))
	//assert.Equal(t, string(b), "{\"field_a\":\"abc\",\"field_b\":5}\n")

	e.Level = log.ErrorLevel
	b, err = layout.ToBytes(e)
	assert.Nil(t, err)
	fmt.Print(string(b))
	//assert.Equal(t, string(b), "{\"field_a\":\"abc\",\"field_b\":5}\n")

	e.Level = log.PanicLevel
	b, err = layout.ToBytes(e)
	assert.Nil(t, err)
	fmt.Print(string(b))
	//assert.Equal(t, string(b), "{\"field_a\":\"abc\",\"field_b\":5}\n")

	e.Level = log.FatalLevel
	b, err = layout.ToBytes(e)
	assert.Nil(t, err)
	fmt.Print(string(b))
	//assert.Equal(t, string(b), "{\"field_a\":\"abc\",\"field_b\":5}\n")
}
