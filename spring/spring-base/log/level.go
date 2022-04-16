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

package log

import "strings"

const (
	NoneLevel  = Level(-1)
	TraceLevel = Level(0)
	DebugLevel = Level(1)
	InfoLevel  = Level(2)
	WarnLevel  = Level(3)
	ErrorLevel = Level(4)
	PanicLevel = Level(5)
	FatalLevel = Level(6)
	OffLevel   = Level(7)
)

// Level 日志输出级别。
type Level int32

func (level Level) String() string {
	switch level {
	case TraceLevel:
		return "trace"
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case PanicLevel:
		return "panic"
	case FatalLevel:
		return "fatal"
	case OffLevel:
		return "off"
	default:
		return "none"
	}
}

func StringToLevel(str string) Level {
	switch strings.ToLower(str) {
	case "trace":
		return TraceLevel
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "warn":
		return WarnLevel
	case "error":
		return ErrorLevel
	case "panic":
		return PanicLevel
	case "fatal":
		return FatalLevel
	case "off":
		return OffLevel
	default:
		return NoneLevel
	}
}
