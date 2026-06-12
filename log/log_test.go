/*
 * Copyright 2025 The Go-Spring Authors.
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
	"bytes"
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-spring/log"
	"github.com/go-spring/stdlib/flatten"
	"github.com/go-spring/stdlib/testing/assert"
)

var (
	keyTraceID int
	keySpanID  int
)

var TagDefault = log.RegisterTag("_def")
var TagRequestIn = log.RegisterTag("_com_request_in")
var TagRequestOut = log.RegisterTag(log.BuildTag("com", "request", "out"))

var rootLogger = log.GetLogger(log.RootLoggerName)

// Loggers with same name 'myLogger'
var myLogger = log.GetLogger("myLogger")
var myLoggerV2 = log.GetLogger("myLogger")

///////////////////////////////////////////////////////////////////////////////

var _ log.Appender = (*SampleAppender)(nil)

type SampleAppender struct {
	log.AppenderBase
	Layout log.Layout `PluginElement:"layout,default=TextLayout"`
}

func (a *SampleAppender) Start() error { return nil }
func (a *SampleAppender) Stop()        {}
func (a *SampleAppender) Append(e *log.Event) {
	log.WriteEvent(os.Stdout, e, a.Layout)
	log.WriteEvent(os.Stderr, e, a.Layout)
}
func (a *SampleAppender) ConcurrentSafe() bool { return true }

func init() {
	log.RegisterPlugin[SampleAppender]("SampleAppender")
}

///////////////////////////////////////////////////////////////////////////////

func readConfig() map[string]string {
	s := `
	{
	  "bufferCap": "1KB",
	  "bufferSize": 1000,
	  "appender": {
	    "file": {
	      "type": "FileAppender",
	      "file": "log.txt",
	      "layout!": "JSONLayout{}"
	    },
	    "console!": "ConsoleAppender{layout=TextLayout{}}",
	    "sample!": "SampleAppender{layout.type=TextLayout}"
	  },
	  "logger": {
	    "root": {
	      "type": "Logger",
	      "level": "warn",
	      "appenderRef": {
	        "ref": "console"
	      }
	    },
	    "myLogger": {
	      "type": "AsyncLogger",
	      "level": "trace",
	      "tag": "_com_request_in,_com_request_*",
	      "bufferSize": "${bufferSize}",
	      "appenderRef": [
	        {
	          "ref": "file"
	        },
	        {
	          "ref": "sample"
	        }
	      ]
	    }
	  }
	}`

	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		panic(err)
	}
	return flatten.Flatten(m)
}

func TestLog(t *testing.T) {
	ctx := t.Context()
	_ = os.Remove("logs/log.txt")

	logBuf := bytes.NewBuffer(nil)
	log.Stdout = logBuf
	defer func() {
		log.Stdout = os.Stdout
	}()

	log.TimeNow = func(ctx context.Context) time.Time {
		return time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	}

	log.StringFromContext = func(ctx context.Context) string {
		return ""
	}

	log.FieldsFromContext = func(ctx context.Context) []log.Field {
		traceID, _ := ctx.Value(&keyTraceID).(string)
		spanID, _ := ctx.Value(&keySpanID).(string)
		return []log.Field{
			log.String("trace_id", traceID),
			log.String("span_id", spanID),
		}
	}

	// not print
	log.Tracef(ctx, TagRequestOut, "hello %s", "world")
	log.Debugf(ctx, TagRequestOut, "hello %s", "world")

	// print
	log.Info(ctx, TagDefault, log.Msgf("hello %s", "world"))
	log.Info(ctx, TagRequestIn, log.Msgf("hello %s", "world"))

	err := log.RefreshConfig(readConfig())
	assert.Error(t, err).Nil()

	ctx = context.WithValue(ctx, &keyTraceID, "0a882193682db71edd48044db54cae88")
	ctx = context.WithValue(ctx, &keySpanID, "50ef0724418c0a66")

	// print
	log.Trace(ctx, TagRequestOut, func() []log.Field {
		return []log.Field{
			log.Msgf("hello %s", "world"),
		}
	})

	// print
	log.Debug(ctx, TagRequestOut, func() []log.Field {
		return []log.Field{
			log.Msgf("hello %s", "world"),
		}
	})

	// print
	log.Tracef(ctx, TagRequestOut, "hello %s", "world")
	log.Debugf(ctx, TagRequestOut, "hello %s", "world")

	// print
	log.Info(ctx, TagRequestIn, log.Msgf("hello %s", "world"))
	log.Warn(ctx, TagRequestIn, log.Msgf("hello %s", "world"))
	log.Error(ctx, TagRequestIn, log.Msgf("hello %s", "world"))
	log.Panic(ctx, TagRequestIn, log.Msgf("hello %s", "world"))
	log.Fatal(ctx, TagRequestIn, log.Msgf("hello %s", "world"))

	// print
	log.Infof(ctx, TagRequestIn, "hello %s", "world")
	log.Warnf(ctx, TagRequestIn, "hello %s", "world")
	log.Errorf(ctx, TagRequestIn, "hello %s", "world")
	log.Panicf(ctx, TagRequestIn, "hello %s", "world")
	log.Fatalf(ctx, TagRequestIn, "hello %s", "world")

	// not print
	log.Info(ctx, TagDefault, log.Msgf("hello %s", "world"))

	// print
	log.Warn(ctx, TagDefault, log.Msgf("hello %s", "world"))
	log.Error(ctx, TagDefault, log.Msgf("hello %s", "world"))
	log.Panic(ctx, TagDefault, log.Msgf("hello %s", "world"))

	// print
	log.Error(ctx, TagDefault, log.FieldsFromMap(map[string]any{
		"key1": "value1",
		"key2": "value2",
	}))

	rootLogger.Write(log.WarnLevel, []byte("this message is written directly\n"))
	rootLogger.Write(log.WarnLevel, []byte("this message is written directly\n"))

	expectLog := `
[INFO][2025-06-01T00:00:00.000][<<file>>:150] _def||trace_id=||span_id=||msg=hello world
[INFO][2025-06-01T00:00:00.000][<<file>>:151] _com_request_in||trace_id=||span_id=||msg=hello world
[WARN][2025-06-01T00:00:00.000][<<file>>:195] _def||trace_id=0a882193682db71edd48044db54cae88||span_id=50ef0724418c0a66||msg=hello world
[ERROR][2025-06-01T00:00:00.000][<<file>>:196] _def||trace_id=0a882193682db71edd48044db54cae88||span_id=50ef0724418c0a66||msg=hello world
[PANIC][2025-06-01T00:00:00.000][<<file>>:197] _def||trace_id=0a882193682db71edd48044db54cae88||span_id=50ef0724418c0a66||msg=hello world
[ERROR][2025-06-01T00:00:00.000][<<file>>:200] _def||trace_id=0a882193682db71edd48044db54cae88||span_id=50ef0724418c0a66||key1=value1||key2=value2
this message is written directly
this message is written directly
`

	_, currFile, _, _ := runtime.Caller(0)
	expectLog = strings.ReplaceAll(expectLog, "<<file>>", currFile)
	assert.String(t, logBuf.String()).Equal(strings.TrimLeft(expectLog, "\n"))

	myLogger.Write(log.InfoLevel, []byte("this message is written directly\n"))
	myLoggerV2.Write(log.InfoLevel, []byte("this message is written directly\n"))

	expectLog = `
{"level":"trace","time":"2025-06-01T00:00:00.000","fileLine":"<<file>>:160","tag":"_com_request_out","trace_id":"0a882193682db71edd48044db54cae88","span_id":"50ef0724418c0a66","msg":"hello world"}
{"level":"debug","time":"2025-06-01T00:00:00.000","fileLine":"<<file>>:167","tag":"_com_request_out","trace_id":"0a882193682db71edd48044db54cae88","span_id":"50ef0724418c0a66","msg":"hello world"}
{"level":"trace","time":"2025-06-01T00:00:00.000","fileLine":"<<file>>:174","tag":"_com_request_out","trace_id":"0a882193682db71edd48044db54cae88","span_id":"50ef0724418c0a66","msg":"hello world"}
{"level":"debug","time":"2025-06-01T00:00:00.000","fileLine":"<<file>>:175","tag":"_com_request_out","trace_id":"0a882193682db71edd48044db54cae88","span_id":"50ef0724418c0a66","msg":"hello world"}
{"level":"info","time":"2025-06-01T00:00:00.000","fileLine":"<<file>>:178","tag":"_com_request_in","trace_id":"0a882193682db71edd48044db54cae88","span_id":"50ef0724418c0a66","msg":"hello world"}
{"level":"warn","time":"2025-06-01T00:00:00.000","fileLine":"<<file>>:179","tag":"_com_request_in","trace_id":"0a882193682db71edd48044db54cae88","span_id":"50ef0724418c0a66","msg":"hello world"}
{"level":"error","time":"2025-06-01T00:00:00.000","fileLine":"<<file>>:180","tag":"_com_request_in","trace_id":"0a882193682db71edd48044db54cae88","span_id":"50ef0724418c0a66","msg":"hello world"}
{"level":"panic","time":"2025-06-01T00:00:00.000","fileLine":"<<file>>:181","tag":"_com_request_in","trace_id":"0a882193682db71edd48044db54cae88","span_id":"50ef0724418c0a66","msg":"hello world"}
{"level":"fatal","time":"2025-06-01T00:00:00.000","fileLine":"<<file>>:182","tag":"_com_request_in","trace_id":"0a882193682db71edd48044db54cae88","span_id":"50ef0724418c0a66","msg":"hello world"}
{"level":"info","time":"2025-06-01T00:00:00.000","fileLine":"<<file>>:185","tag":"_com_request_in","trace_id":"0a882193682db71edd48044db54cae88","span_id":"50ef0724418c0a66","msg":"hello world"}
{"level":"warn","time":"2025-06-01T00:00:00.000","fileLine":"<<file>>:186","tag":"_com_request_in","trace_id":"0a882193682db71edd48044db54cae88","span_id":"50ef0724418c0a66","msg":"hello world"}
{"level":"error","time":"2025-06-01T00:00:00.000","fileLine":"<<file>>:187","tag":"_com_request_in","trace_id":"0a882193682db71edd48044db54cae88","span_id":"50ef0724418c0a66","msg":"hello world"}
{"level":"panic","time":"2025-06-01T00:00:00.000","fileLine":"<<file>>:188","tag":"_com_request_in","trace_id":"0a882193682db71edd48044db54cae88","span_id":"50ef0724418c0a66","msg":"hello world"}
{"level":"fatal","time":"2025-06-01T00:00:00.000","fileLine":"<<file>>:189","tag":"_com_request_in","trace_id":"0a882193682db71edd48044db54cae88","span_id":"50ef0724418c0a66","msg":"hello world"}
this message is written directly
this message is written directly
`

	// Since an asynchronous logger is used, it is necessary to call stop first
	// before performing the assertion to ensure all log entries are flushed and
	// the logger is properly stopped.
	log.Destroy()

	b, err := os.ReadFile("logs/log.txt")
	assert.Error(t, err).Nil()
	expectLog = strings.ReplaceAll(expectLog, "<<file>>", currFile)
	assert.String(t, string(b)).Equal(strings.TrimLeft(expectLog, "\n"))
}
