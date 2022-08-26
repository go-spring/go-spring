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

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-spring/spring-base/color"
	"github.com/go-spring/spring-base/util"
)

func init() {
	RegisterPlugin("JSONLayout", PluginTypeLayout, (*JSONLayout)(nil))
	RegisterPlugin("DefaultLayout", PluginTypeLayout, (*DefaultLayout)(nil))
}

// Layout lays out an Event in []byte format.
type Layout interface {
	ToBytes(e *Event) ([]byte, error)
}

type ColorStyle int

const (
	ColorStyleNone = ColorStyle(iota)
	ColorStyleNormal
	ColorStyleBright
)

func ParseColorStyle(s string) (ColorStyle, error) {
	switch strings.ToLower(s) {
	case "none":
		return ColorStyleNone, nil
	case "normal":
		return ColorStyleNormal, nil
	case "bright":
		return ColorStyleBright, nil
	default:
		return -1, fmt.Errorf("invalid color style %s", s)
	}
}

type FormatFunc func(e *Event) string

type DefaultLayout struct {
	LineBreak  bool       `PluginAttribute:"lineBreak,default=true"`
	ColorStyle ColorStyle `PluginAttribute:"colorStyle,default=none"`
	Formatter  string     `PluginAttribute:"formatter,default="`
	steps      []FormatFunc
}

func (c *DefaultLayout) Init() error {
	if c.Formatter == "" {
		c.Formatter = "[:level][:time][:fileline][:msg]"
	}
	return c.parse(c.Formatter)
}

func (c *DefaultLayout) ToBytes(e *Event) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	for _, step := range c.steps {
		buf.WriteString(step(e))
	}
	if c.LineBreak {
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func (c *DefaultLayout) parse(formatter string) error {
	write := func(s string) FormatFunc {
		return func(e *Event) string {
			return s
		}
	}
	c.steps = append(c.steps, write("["))
	c.steps = append(c.steps, c.getLevel)
	c.steps = append(c.steps, write("]"))
	c.steps = append(c.steps, write("["))
	c.steps = append(c.steps, c.getTime)
	c.steps = append(c.steps, write("]"))
	c.steps = append(c.steps, write("["))
	c.steps = append(c.steps, c.getFileLine)
	c.steps = append(c.steps, write("]"))
	c.steps = append(c.steps, write(" "))
	c.steps = append(c.steps, c.getMsg)
	return nil
}

func (c *DefaultLayout) getMsg(e *Event) string {
	buf := bytes.NewBuffer(nil)
	if tag := e.entry.Tag(); tag != "" {
		buf.WriteString(tag)
		buf.WriteString("||")
	}
	enc := NewFlatEncoder(buf, "||")
	err := enc.AppendEncoderBegin()
	if err != nil {
		return err.Error()
	}
	for _, f := range e.fields {
		err = enc.AppendKey(f.Key)
		if err != nil {
			return err.Error()
		}
		err = f.Val.Encode(enc)
		if err != nil {
			return err.Error()
		}
	}
	err = enc.AppendEncoderEnd()
	if err != nil {
		return err.Error()
	}
	return buf.String()
}

func (c *DefaultLayout) getLevel(e *Event) string {
	level := e.Level()
	strLevel := strings.ToUpper(level.String())
	switch c.ColorStyle {
	case ColorStyleNormal:
		if level >= ErrorLevel {
			strLevel = color.Red.Sprint(strLevel)
		} else if level == WarnLevel {
			strLevel = color.Yellow.Sprint(strLevel)
		} else if level <= DebugLevel {
			strLevel = color.Green.Sprint(strLevel)
		}
	}
	return strLevel
}

func (c *DefaultLayout) getTime(e *Event) string {
	return e.Time().Format("2006-01-02T15:04:05.000")
}

func (c *DefaultLayout) getFileLine(e *Event) string {
	return util.Contract(fmt.Sprintf("%s:%d", e.File(), e.Line()), 48)
}

type JSONLayout struct {
	LineBreak bool `PluginAttribute:"lineBreak,default=true"`
}

func (c *JSONLayout) ToBytes(e *Event) ([]byte, error) {

	buf := bytes.NewBuffer(nil)
	enc := NewJSONEncoder(buf)
	err := enc.AppendEncoderBegin()
	if err != nil {
		return nil, err
	}
	for _, f := range e.fields {
		err = enc.AppendKey(f.Key)
		if err != nil {
			return nil, err
		}
		err = f.Val.Encode(enc)
		if err != nil {
			return nil, err
		}
	}
	err = enc.AppendEncoderEnd()
	if err != nil {
		return nil, err
	}
	if c.LineBreak {
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}
