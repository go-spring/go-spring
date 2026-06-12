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
	"testing"

	"github.com/go-spring/stdlib/errutil"
)

func TestParseHumanizeBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr error
	}{
		{
			name:  "kilobytes",
			input: "1KB",
			want:  1024,
		},
		{
			name:  "case insensitive",
			input: "1kb",
			want:  1024,
		},
		{
			name:  "space before unit",
			input: "1 KB",
			want:  1024,
		},
		{
			name:  "space after unit",
			input: "1KB ",
			want:  1024,
		},
		{
			name:    "invalid number",
			input:   "abcKB",
			wantErr: errutil.Explain(nil, `strconv.ParseInt: parsing "": invalid syntax`),
		},
		{
			name:    "missing unit",
			input:   "1024",
			wantErr: errutil.Explain(nil, `invalid unit ""`),
		},
		{
			name:    "unknown unit",
			input:   "1GB",
			wantErr: errutil.Explain(nil, `invalid unit "GB"`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHumanizeBytes(tt.input)
			if err != nil && err.Error() != tt.wantErr.Error() {
				t.Errorf("ParseHumanizeBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseHumanizeBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBaseLayout(t *testing.T) {
	tests := []struct {
		name              string
		fileLineMaxLength int
		file              string
		line              int
		want              string
	}{
		{
			name:              "normal file line",
			fileLineMaxLength: 48,
			file:              "file.go",
			line:              100,
			want:              "file.go:100",
		},
		{
			name:              "long file line truncated",
			fileLineMaxLength: 20,
			file:              "very/long/path/to/file.go",
			line:              100,
			want:              "...th/to/file.go:100",
		},
		{
			name:              "exact length file line",
			fileLineMaxLength: 13,
			file:              "file.go",
			line:              100,
			want:              "file.go:100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &BaseLayout{
				FileLineMaxLength: tt.fileLineMaxLength,
			}
			e := &Event{
				File: tt.file,
				Line: tt.line,
			}
			if got := l.GetFileLine(e); got != tt.want {
				t.Errorf("BaseLayout.GetFileLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

//func TestTextLayout(t *testing.T) {
//
//	t.Run("without ctx string & fields", func(t *testing.T) {
//		layout := &TextLayout{
//			BaseLayout{
//				FileLineLength: 48,
//			},
//		}
//		b := layout.ToBytes(&Event{
//			Level:  InfoLevel,
//			Time:   time.Time{},
//			File:   "file.go",
//			Line:   100,
//			Tag:    "_def",
//			Fields: []Field{Msg("hello world")},
//		})
//		assert.String(t, string(b)).Equal("[INFO][0001-01-01T00:00:00.000][file.go:100] _def||msg=hello world\n")
//	})
//
//	t.Run("with ctx string", func(t *testing.T) {
//		layout := &TextLayout{
//			BaseLayout{
//				FileLineLength: 48,
//			},
//		}
//		b := layout.ToBytes(&Event{
//			Level:     InfoLevel,
//			Time:      time.Time{},
//			File:      "gs/examples/bookman/src/biz/service/book_service/book_service_test.go",
//			Line:      100,
//			Tag:       "_def",
//			Fields:    []Field{Msg("hello world")},
//			CtxString: "trace_id=0a882193682db71edd48044db54cae88||span_id=50ef0724418c0a66",
//			CtxFields: nil,
//		})
//		assert.String(t, string(b)).Equal("[INFO][0001-01-01T00:00:00.000][...service/book_service/book_service_test.go:100] _def||trace_id=0a882193682db71edd48044db54cae88||span_id=50ef0724418c0a66||msg=hello world\n")
//	})
//
//	t.Run("with ctx fields", func(t *testing.T) {
//		layout := &TextLayout{
//			BaseLayout{
//				FileLineLength: 48,
//			},
//		}
//		b := layout.ToBytes(&Event{
//			Level:     InfoLevel,
//			Time:      time.Time{},
//			File:      "file.go",
//			Line:      100,
//			Tag:       "_def",
//			Fields:    []Field{Msg("hello world")},
//			CtxFields: []Field{String("key", "value")},
//		})
//		assert.String(t, string(b)).Equal("[INFO][0001-01-01T00:00:00.000][file.go:100] _def||key=value||msg=hello world\n")
//	})
//}
//
//func TestJSONLayout(t *testing.T) {
//
//	t.Run("without ctx string & fields", func(t *testing.T) {
//		layout := &JSONLayout{
//			BaseLayout{
//				FileLineLength: 48,
//			},
//		}
//		b := layout.ToBytes(&Event{
//			Level:  InfoLevel,
//			Time:   time.Time{},
//			File:   "file.go",
//			Line:   100,
//			Tag:    "_def",
//			Fields: []Field{Msg("hello world")},
//		})
//		assert.String(t, string(b)).Equal(`{"level":"info","time":"0001-01-01T00:00:00.000","fileLine":"file.go:100","tag":"_def","msg":"hello world"}` + "\n")
//	})
//
//	t.Run("with ctx string", func(t *testing.T) {
//		layout := &JSONLayout{
//			BaseLayout{
//				FileLineLength: 48,
//			},
//		}
//		b := layout.ToBytes(&Event{
//			Level:     InfoLevel,
//			Time:      time.Time{},
//			File:      "gs/examples/bookman/src/biz/service/book_service/book_service_test.go",
//			Line:      100,
//			Tag:       "_def",
//			Fields:    []Field{Msg("hello world")},
//			CtxString: "trace_id=0a882193682db71edd48044db54cae88||span_id=50ef0724418c0a66",
//			CtxFields: nil,
//		})
//		assert.String(t, string(b)).Equal(`{"level":"info","time":"0001-01-01T00:00:00.000","fileLine":"...service/book_service/book_service_test.go:100","tag":"_def","ctxString":"trace_id=0a882193682db71edd48044db54cae88||span_id=50ef0724418c0a66","msg":"hello world"}` + "\n")
//	})
//
//	t.Run("with ctx fields", func(t *testing.T) {
//		layout := &JSONLayout{
//			BaseLayout{
//				FileLineLength: 48,
//			},
//		}
//		b := layout.ToBytes(&Event{
//			Level:     InfoLevel,
//			Time:      time.Time{},
//			File:      "file.go",
//			Line:      100,
//			Tag:       "_def",
//			Fields:    []Field{Msg("hello world")},
//			CtxFields: []Field{String("key", "value")},
//		})
//		assert.String(t, string(b)).Equal(`{"level":"info","time":"0001-01-01T00:00:00.000","fileLine":"file.go:100","tag":"_def","key":"value","msg":"hello world"}` + "\n")
//	})
//}
