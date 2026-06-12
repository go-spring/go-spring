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

package log

import (
	"encoding/json"
	"os"
	"testing"

	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

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

func TestGetLogger(t *testing.T) {
	l := GetLogger("logger-not-exist")
	err := RefreshConfig(readConfig())
	assert.Error(t, err).Matches(`logger logger-not-exist not found`)
	delete(loggerMap, l.name)
	Destroy()
}

func TestGetLoggerBeforeRefreshUsesDefaultLogger(t *testing.T) {
	logBuf, err := os.CreateTemp(os.TempDir(), "")
	assert.Error(t, err).Nil()
	t.Cleanup(func() {
		_ = logBuf.Close()
		_ = os.Remove(logBuf.Name())
		Stdout = os.Stdout
		delete(loggerMap, "beforeRefresh")
	})

	Stdout = logBuf
	l := GetLogger("beforeRefresh")
	l.Write(InfoLevel, []byte("hello\n"))

	_, err = logBuf.Seek(0, 0)
	assert.Error(t, err).Nil()
	b, err := os.ReadFile(logBuf.Name())
	assert.Error(t, err).Nil()
	assert.String(t, string(b)).Equal("hello\n")
}
