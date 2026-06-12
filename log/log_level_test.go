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
	"github.com/go-spring/stdlib/testing/assert"
)

func TestRegisterLevel(t *testing.T) {
	customLevel := RegisterLevel(800, "custom")
	assert.Number(t, customLevel.Code()).Equal(int32(800))
	assert.String(t, customLevel.UpperName()).Equal("CUSTOM")
}

func TestParseLevelRange(t *testing.T) {
	tests := []struct {
		str     string
		want    LevelRange
		wantErr error
	}{
		{
			str:  "none",
			want: LevelRange{MinLevel: NoneLevel, MaxLevel: MaxLevel},
		},
		{
			str:  "trace",
			want: LevelRange{MinLevel: TraceLevel, MaxLevel: MaxLevel},
		},
		{
			str:  "debug",
			want: LevelRange{MinLevel: DebugLevel, MaxLevel: MaxLevel},
		},
		{
			str:  "info",
			want: LevelRange{MinLevel: InfoLevel, MaxLevel: MaxLevel},
		},
		{
			str:  "warn",
			want: LevelRange{MinLevel: WarnLevel, MaxLevel: MaxLevel},
		},
		{
			str:  "error",
			want: LevelRange{MinLevel: ErrorLevel, MaxLevel: MaxLevel},
		},
		{
			str:  "panic",
			want: LevelRange{MinLevel: PanicLevel, MaxLevel: MaxLevel},
		},
		{
			str:  "fatal",
			want: LevelRange{MinLevel: FatalLevel, MaxLevel: MaxLevel},
		},
		{
			str:     "unknown",
			want:    LevelRange{},
			wantErr: errutil.Explain(nil, "invalid log level: %q", "unknown"),
		},
	}
	for _, tt := range tests {
		got, err := ParseLevelRange(tt.str)
		assert.That(t, got).Equal(tt.want)
		assert.That(t, err).Equal(tt.wantErr)
	}

	// Test that levels are properly ordered by code
	assert.Number(t, NoneLevel.Code()).LessThan(TraceLevel.Code())
	assert.Number(t, TraceLevel.Code()).LessThan(DebugLevel.Code())
	assert.Number(t, DebugLevel.Code()).LessThan(InfoLevel.Code())
	assert.Number(t, InfoLevel.Code()).LessThan(WarnLevel.Code())
	assert.Number(t, WarnLevel.Code()).LessThan(ErrorLevel.Code())
	assert.Number(t, ErrorLevel.Code()).LessThan(PanicLevel.Code())
	assert.Number(t, PanicLevel.Code()).LessThan(FatalLevel.Code())
}
