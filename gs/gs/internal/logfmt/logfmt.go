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

// Package logfmt colorizes log lines by their [LEVEL] prefix when the
// process writes to a real terminal. On pipes, files, or when NO_COLOR is
// set, output is passed through unchanged so machine-readable consumers
// (grep, less -R off, CI logs) see plain text.
package logfmt

import (
	"bytes"
	"io"
	"log"
	"os"
)

const (
	reset = "\x1b[0m"
	cyan  = "\x1b[36m" // [INFO]
	gray  = "\x1b[2m"  // [DEBUG]
)

// levels is a fixed-order slice so lines matching multiple prefixes (they
// shouldn't) fall on a predictable one; each entry pairs the literal tag
// with the ANSI sequence that colors it.
var levels = []struct {
	tag  []byte
	code string
}{
	{[]byte("[INFO]"), cyan},
	{[]byte("[DEBUG]"), gray},
}

// Setup routes the default logger through a TTY-aware colorizer.
// Callers should invoke it once at program start, before any log output.
func Setup() {
	if colorEnabled() {
		log.SetOutput(colorWriter{})
	}
}

func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// colorWriter wraps the first matching level tag in an ANSI sequence and
// forwards the rest of the line untouched. The reported byte count matches
// the input so log's internal accounting stays correct despite the extra
// escape bytes on the wire.
type colorWriter struct{}

func (colorWriter) Write(p []byte) (int, error) {
	out := p
	for _, l := range levels {
		if i := bytes.Index(out, l.tag); i >= 0 {
			colored := make([]byte, 0, len(out)+len(l.code)+len(reset))
			colored = append(colored, out[:i]...)
			colored = append(colored, l.code...)
			colored = append(colored, l.tag...)
			colored = append(colored, reset...)
			colored = append(colored, out[i+len(l.tag):]...)
			out = colored
			break
		}
	}
	if _, err := io.Writer(os.Stderr).Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}
