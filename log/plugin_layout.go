/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * You may not use this file except in compliance with the License.
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
	"strconv"
)

func init() {
	RegisterPlugin[TextLayout]("TextLayout")
	RegisterPlugin[JSONLayout]("JSONLayout")
}

// Layout defines how a log event is encoded into a writer.
// Implementations should write fully formatted log data to `w`.
// Layouts do NOT manage memory or buffering; callers are responsible.
type Layout interface {
	EncodeTo(e *Event, w Writer)
}

// BaseLayout provides common utilities for layouts, e.g., file:line formatting.
type BaseLayout struct {
	FileLineMaxLength int `PluginAttribute:"fileLineMaxLength,default=48"`
}

// GetFileLine returns the "file:line" string for a log event.
// If the result exceeds FileLineMaxLength,
// the leading part is truncated and replaced with "...".
func (c *BaseLayout) GetFileLine(e *Event) string {
	fileLine := e.File + ":" + strconv.Itoa(e.Line)
	if c.FileLineMaxLength <= 16 {
		return fileLine
	}
	if n := len(fileLine); n > c.FileLineMaxLength {
		fileLine = "..." + fileLine[n-c.FileLineMaxLength+3:]
	}
	return fileLine
}

// TextLayout encodes a log event as a human-readable text line.
type TextLayout struct {
	BaseLayout
}

// EncodeTo writes the log event to the provided writer in plain-text format.
func (c *TextLayout) EncodeTo(e *Event, w Writer) {
	const separator = "||"

	// Write basic header fields
	_, _ = w.WriteString("[")
	_, _ = w.WriteString(e.Level.UpperName())
	_, _ = w.WriteString("][")
	_, _ = w.WriteString(e.Time.Format("2006-01-02T15:04:05.000"))
	_, _ = w.WriteString("][")
	_, _ = w.WriteString(c.GetFileLine(e))
	_, _ = w.WriteString("] ")
	_, _ = w.WriteString(e.Tag)
	_, _ = w.WriteString(separator)
	if e.CtxString != "" {
		_, _ = w.WriteString(e.CtxString)
		_, _ = w.WriteString(separator)
	}

	// Encode structured fields
	enc := NewTextEncoder(w, separator)
	enc.AppendEncoderBegin()
	EncodeFields(enc, e.CtxFields)
	EncodeFields(enc, e.Fields)
	enc.AppendEncoderEnd()

	_ = w.WriteByte('\n')
}

// JSONLayout encodes a log event as a structured JSON object.
type JSONLayout struct {
	BaseLayout
}

// EncodeTo writes the log event to the provided writer in JSON format.
func (c *JSONLayout) EncodeTo(e *Event, w Writer) {
	enc := NewJSONEncoder(w)
	enc.AppendEncoderBegin()

	// Write basic header fields
	String("level", e.Level.LowerName()).Encode(enc)
	String("time", e.Time.Format("2006-01-02T15:04:05.000")).Encode(enc)
	String("fileLine", c.GetFileLine(e)).Encode(enc)
	String("tag", e.Tag).Encode(enc)
	if e.CtxString != "" {
		String("ctxString", e.CtxString).Encode(enc)
	}

	// Encode structured fields
	EncodeFields(enc, e.CtxFields)
	EncodeFields(enc, e.Fields)
	enc.AppendEncoderEnd()

	_ = w.WriteByte('\n')
}
