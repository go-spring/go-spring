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
	"reflect"
	"runtime"
)

const (
	PluginTypeAppender = "Appender"
	PluginTypeFilter   = "Filter"
	PluginTypeLayout   = "Layout"
)

// plugins stores user registered Plugin(s) .
var plugins = map[string]*Plugin{}

// Plugin is the name of node label or XML element.
type Plugin struct {
	Name  string
	Type  string
	Class reflect.Type
	File  string
	Line  int
}

// RegisterPlugin registers a Plugin, `i` is used to obtain the type of Plugin.
func RegisterPlugin(name string, typ string, i interface{}) {
	_, file, line, _ := runtime.Caller(1)
	if p, ok := plugins[name]; ok {
		panic(fmt.Errorf("duplicate plugin %s in %s:%d and %s:%d", typ, p.File, p.Line, file, line))
	}
	class := reflect.TypeOf(i).Elem()
	p := &Plugin{
		Name:  name,
		Type:  typ,
		Class: class,
		File:  file,
		Line:  line,
	}
	plugins[name] = p
}
