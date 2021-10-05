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

// Package color 提供了一些控制台输出格式。
package color

import (
	"bytes"
	"fmt"
)

const (
	Bold         Attribute = "1" // 粗体
	Italic       Attribute = "3" // 斜体
	Underline    Attribute = "4" // 下划线
	ReverseVideo Attribute = "7" // 反色
	CrossedOut   Attribute = "9" // 删除线
)

const (
	Black   Attribute = "30" // 黑色
	Red     Attribute = "31" // 红色
	Green   Attribute = "32" // 绿色
	Yellow  Attribute = "33" // 黄色
	Blue    Attribute = "34" // 蓝色
	Magenta Attribute = "35" // 紫色
	Cyan    Attribute = "36" // 青色
	White   Attribute = "37" // 白色
)

const (
	BgBlack   Attribute = "40" // 背景黑色
	BgRed     Attribute = "41" // 背景红色
	BgGreen   Attribute = "42" // 背景绿色
	BgYellow  Attribute = "43" // 背景黄色
	BgBlue    Attribute = "44" // 背景蓝色
	BgMagenta Attribute = "45" // 背景紫色
	BgCyan    Attribute = "46" // 背景青色
	BgWhite   Attribute = "47" // 背景白色
)

// Attribute 控制台属性
type Attribute string

// Sprint 返回根据控制台属性格式化后的字符串。
func (attr Attribute) Sprint(a ...interface{}) string {
	return wrap([]Attribute{attr}, fmt.Sprint(a...))
}

// Sprintf 返回根据控制台属性格式化后的字符串。
func (attr Attribute) Sprintf(format string, a ...interface{}) string {
	return wrap([]Attribute{attr}, fmt.Sprintf(format, a...))
}

// Text 控制台属性组合
type Text struct {
	attributes []Attribute
}

// NewText 返回组合的控制台属性。
func NewText(attributes ...Attribute) *Text {
	return &Text{attributes: attributes}
}

// Sprint 返回根据控制台属性格式化后的字符串。
func (c *Text) Sprint(a ...interface{}) string {
	return wrap(c.attributes, fmt.Sprint(a...))
}

// Sprintf 返回根据控制台属性格式化后的字符串。
func (c *Text) Sprintf(format string, a ...interface{}) string {
	return wrap(c.attributes, fmt.Sprintf(format, a...))
}

// wrap 返回根据控制台属性格式化后的字符串。
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
