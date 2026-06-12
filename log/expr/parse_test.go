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

package expr

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "simple type with no fields",
			input: "Logger {}",
			want: map[string]string{
				"type": "Logger",
			},
		},
		{
			name:  "type with string field",
			input: `Logger { level = "info" }`,
			want: map[string]string{
				"type":  "Logger",
				"level": "info",
			},
		},
		{
			name:  "type with raw value field",
			input: "Logger { level = info }",
			want: map[string]string{
				"type":  "Logger",
				"level": "info",
			},
		},
		{
			name:  "type with multiple fields",
			input: `Logger { level = "info", output = "stdout" }`,
			want: map[string]string{
				"type":   "Logger",
				"level":  "info",
				"output": "stdout",
			},
		},
		{
			name:  "type with nested expression",
			input: `Logger { level = "info", file = FileAppender { path = "/tmp/app.log" } }`,
			want: map[string]string{
				"type":      "Logger",
				"level":     "info",
				"file.type": "FileAppender",
				"file.path": "/tmp/app.log",
			},
		},
		{
			name:  "complex nested structure",
			input: `Logger { level = "debug", file = RollingFileAppender { path = "/tmp/app.log", policy = SizeBasedTriggeringPolicy { maxFileSize = "10MB" } } }`,
			want: map[string]string{
				"type":                    "Logger",
				"level":                   "debug",
				"file.type":               "RollingFileAppender",
				"file.path":               "/tmp/app.log",
				"file.policy.type":        "SizeBasedTriggeringPolicy",
				"file.policy.maxFileSize": "10MB",
			},
		},
		{
			name:    "invalid syntax missing closing brace",
			input:   `Logger { level = "info" `,
			wantErr: true,
		},
		{
			name:    "invalid syntax missing equals",
			input:   `Logger { level "info" }`,
			wantErr: true,
		},
		{
			name:    "invalid syntax missing opening brace",
			input:   `Logger level = "info" }`,
			wantErr: true,
		},
		{
			name:    "invalid syntax missing expression type",
			input:   `{host: localhost}`,
			wantErr: true,
		},
		{
			name:  "fields with special characters in strings",
			input: `Logger { format = "time=\"${timestamp}\" level=${level}" }`,
			want: map[string]string{
				"type":   "Logger",
				"format": "time=\"${timestamp}\" level=${level}",
			},
		},
		{
			name:  "whitespace handling",
			input: `  Logger  {  level  =  "info"  }  `,
			want: map[string]string{
				"type":  "Logger",
				"level": "info",
			},
		},
		{
			name:  "field with array index access",
			input: `Logger { appender[0] = "stdout" }`,
			want: map[string]string{
				"type":        "Logger",
				"appender[0]": "stdout",
			},
		},
		{
			name:  "field with dot notation access",
			input: `Logger { appender.out = "stdout" }`,
			want: map[string]string{
				"type":         "Logger",
				"appender.out": "stdout",
			},
		},
		{
			name:  "field with complex access",
			input: `Logger { appender.out[0].name = "stdout" }`,
			want: map[string]string{
				"type":                 "Logger",
				"appender.out[0].name": "stdout",
			},
		},
		{
			name:    "single quoted string",
			input:   `Logger { level = 'info' }`,
			wantErr: true,
		},
		{
			name:  "string with escaped characters",
			input: `Logger { format = "time=\"${timestamp}\"\nlevel=${level}" }`,
			want: map[string]string{
				"type":   "Logger",
				"format": "time=\"${timestamp}\"\nlevel=${level}",
			},
		},
		{
			name:  "trailing comma in field list",
			input: `Logger { level = "info", output = "stdout", }`,
			want: map[string]string{
				"type":   "Logger",
				"level":  "info",
				"output": "stdout",
			},
		},
		{
			name:  "field with multiple dots",
			input: `File{file="log.txt", layout.type=JSONLayout}`,
			want: map[string]string{
				"type":        "File",
				"file":        "log.txt",
				"layout.type": "JSONLayout",
			},
		},
		{
			name:  "integer value",
			input: `Logger { maxFileSize = 1024 }`,
			want: map[string]string{
				"type":        "Logger",
				"maxFileSize": "1024",
			},
		},
		{
			name:  "negative integer value",
			input: `Logger { minLevel = -1 }`,
			want: map[string]string{
				"type":     "Logger",
				"minLevel": "-1",
			},
		},
		{
			name:  "float value",
			input: `Logger { ratio = 0.5 }`,
			want: map[string]string{
				"type":  "Logger",
				"ratio": "0.5",
			},
		},
		{
			name:  "scientific notation float value",
			input: `Logger { threshold = 1e-5 }`,
			want: map[string]string{
				"type":      "Logger",
				"threshold": "1e-5",
			},
		},
		{
			name:  "hexadecimal value",
			input: `Logger { color = 0xFF0000 }`,
			want: map[string]string{
				"type":  "Logger",
				"color": "0xFF0000",
			},
		},
		{
			name:  "deeply nested structure",
			input: `Logger { appender = ConsoleAppender { encoder = PatternEncoder { pattern = "%d %level %msg" } } }`,
			want: map[string]string{
				"type":                     "Logger",
				"appender.type":            "ConsoleAppender",
				"appender.encoder.type":    "PatternEncoder",
				"appender.encoder.pattern": "%d %level %msg",
			},
		},
		{
			name:    "invalid syntax extra comma",
			input:   `Logger { level = "info", , output = "stdout" }`,
			wantErr: true,
		},
		{
			name:    "invalid syntax missing value",
			input:   `Logger { level = }`,
			wantErr: true,
		},
		{
			name:    "duplicate field",
			input:   `Logger { level = "info", level = "debug" }`,
			wantErr: true,
		},
		{
			name:    "duplicate nested field",
			input:   `Logger { appender = ConsoleAppender { file = "a.log" }, appender.file = "b.log" }`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && strings.Contains(err.Error(), "[PANIC]") {
				t.Errorf("Parse() error contains panic details: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() = %v, want %v", got, tt.want)
			}
		})
	}
}
