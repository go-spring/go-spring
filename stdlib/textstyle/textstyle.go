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

package textstyle

import (
	"bytes"
	"fmt"
)

// Attribute represents a console text attribute (color or style).
type Attribute string

const (
	Bold         Attribute = "1"
	Italic       Attribute = "3"
	Underline    Attribute = "4"
	ReverseVideo Attribute = "7"
	CrossedOut   Attribute = "9"
)

// Foreground colors
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

// Background colors
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

// Sprint formats a string with this Attribute.
func (attr Attribute) Sprint(a ...any) string {
	return wrap([]Attribute{attr}, fmt.Sprint(a...))
}

// Sprintf formats a string with this Attribute using fmt.Sprintf syntax.
func (attr Attribute) Sprintf(format string, a ...any) string {
	return wrap([]Attribute{attr}, fmt.Sprintf(format, a...))
}

// Text represents a collection of console text attributes.
type Text struct {
	attributes []Attribute
}

// NewText creates a new Text with the given attributes.
func NewText(attributes ...Attribute) *Text {
	return &Text{attributes: attributes}
}

// Sprint formats a string using the Text's attributes.
func (c *Text) Sprint(a ...any) string {
	return wrap(c.attributes, fmt.Sprint(a...))
}

// Sprintf formats a string using the Text's attributes and fmt.Sprintf syntax.
func (c *Text) Sprintf(format string, a ...any) string {
	return wrap(c.attributes, fmt.Sprintf(format, a...))
}

// wrap wraps the given string with ANSI escape codes for the provided attributes.
func wrap(attributes []Attribute, str string) string {
	if len(attributes) == 0 {
		return str
	}

	var buf bytes.Buffer
	buf.WriteString("\x1b[") // Start ANSI escape code

	for i := range attributes {
		buf.WriteString(string(attributes[i]))
		if i < len(attributes)-1 {
			buf.WriteByte(';') // Separate multiple attributes with ';'
		}
	}

	buf.WriteByte('m')         // End of ANSI escape code
	buf.WriteString(str)       // Add the actual string
	buf.WriteString("\x1b[0m") // Reset all attributes
	return buf.String()
}
