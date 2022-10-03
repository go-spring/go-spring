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

// Package color provides some console output formats.
package color

import (
	"bytes"
	"fmt"
)

const (
	Bold         Attribute = "1"
	Italic       Attribute = "3"
	Underline    Attribute = "4"
	ReverseVideo Attribute = "7"
	CrossedOut   Attribute = "9"
)

const (
	Black   Attribute = "30"
	Red     Attribute = "31"
	Green   Attribute = "32"
	Yellow  Attribute = "33"
	Blue    Attribute = "34"
	Magenta Attribute = "35"
	Cyan    Attribute = "36"
	White   Attribute = "37"
)

const (
	BgBlack   Attribute = "40"
	BgRed     Attribute = "41"
	BgGreen   Attribute = "42"
	BgYellow  Attribute = "43"
	BgBlue    Attribute = "44"
	BgMagenta Attribute = "45"
	BgCyan    Attribute = "46"
	BgWhite   Attribute = "47"
)

type Attribute string

// Sprint returns a string formatted according to console properties.
func (attr Attribute) Sprint(a ...interface{}) string {
	return wrap([]Attribute{attr}, fmt.Sprint(a...))
}

// Sprintf returns a string formatted according to console properties.
func (attr Attribute) Sprintf(format string, a ...interface{}) string {
	return wrap([]Attribute{attr}, fmt.Sprintf(format, a...))
}

type Text struct {
	attributes []Attribute
}

// NewText returns a new *Text.
func NewText(attributes ...Attribute) *Text {
	return &Text{attributes: attributes}
}

// Sprint returns a string formatted according to console properties.
func (c *Text) Sprint(a ...interface{}) string {
	return wrap(c.attributes, fmt.Sprint(a...))
}

// Sprintf returns a string formatted according to console properties.
func (c *Text) Sprintf(format string, a ...interface{}) string {
	return wrap(c.attributes, fmt.Sprintf(format, a...))
}

func wrap(attributes []Attribute, str string) string {
	if len(attributes) == 0 {
		return str
	}
	var buf bytes.Buffer
	buf.WriteString("\x1b[")
	for i := 0; i < len(attributes); i++ {
		buf.WriteString(string(attributes[i]))
		if i < len(attributes)-1 {
			buf.WriteByte(';')
		}
	}
	buf.WriteByte('m')
	buf.WriteString(str)
	buf.WriteString("\x1b[0m")
	return buf.String()
}
