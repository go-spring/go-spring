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
	"fmt"
	"strings"

	"github.com/go-spring/spring-base/color"
	"github.com/go-spring/spring-base/util"
)

func init() {
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

type DefaultLayout struct {
	LineBreak  bool       `PluginAttribute:"lineBreak,default=true"`
	ColorStyle ColorStyle `PluginAttribute:"colorStyle,default=none"`
}

func (c *DefaultLayout) ToBytes(e *Event) ([]byte, error) {
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
	strTime := e.Time().Format("2006-01-02T15:04:05.000")
	fileLine := util.Contract(fmt.Sprintf("%s:%d", e.File(), e.Line()), 48)
	format := "[%s][%s][%s] %s"
	if c.LineBreak {
		format += "\n"
	}
	data := fmt.Sprintf(format, strLevel, strTime, fileLine, e.Msg().Text())
	return []byte(data), nil
}
