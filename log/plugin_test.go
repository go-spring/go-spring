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
	"reflect"
	"testing"

	"github.com/go-spring/stdlib/flatten"
	"github.com/go-spring/stdlib/testing/assert"
)

func TestRegisterPlugin(t *testing.T) {
	assert.Panic(t, func() {
		RegisterPlugin[int]("DummyLayout")
	}, "T must be struct")
	assert.Panic(t, func() {
		RegisterPlugin[FileAppender]("FileAppender")
	}, "duplicate plugin name \"FileAppender\" in .*/plugin_appender.go:.* and .*/plugin_test.go:.*")
}

func TestInjectAttribute(t *testing.T) {

	t.Run("no attribute - 1", func(t *testing.T) {
		type ErrorPlugin struct {
			Name string `PluginAttribute:""`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		_, err := newPlugin(typ, "test", nil)
		assert.Error(t, err).Matches("PluginAttribute tag is empty for field at test")
	})

	t.Run("no attribute - 2", func(t *testing.T) {
		type ErrorPlugin struct {
			Value string `PluginAttribute:"value"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches("inject field ErrorPlugin.Value error >> no value configured and no default specified")
	})

	t.Run("property not found error", func(t *testing.T) {
		type ErrorPlugin struct {
			Value string `PluginAttribute:"value"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.value", "${nonexistent_prop}")
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`property reference "\${nonexistent_prop}" does not exist`)
	})

	t.Run("converter error", func(t *testing.T) {
		type ErrorPlugin struct {
			Level LevelRange `PluginAttribute:"level,default=NOT-EXIST-LEVEL"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches("inject field ErrorPlugin.Level error >> invalid log level: \"NOT-EXIST-LEVEL\"")
	})

	t.Run("uint64 error", func(t *testing.T) {
		type ErrorPlugin struct {
			M uint64 `PluginAttribute:"m,default=111"`
			N uint64 `PluginAttribute:"n,default=abc"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.N error >> parse "abc" to uint64 error: strconv.ParseUint: parsing "abc": invalid syntax`)
	})

	t.Run("int64 error", func(t *testing.T) {
		type ErrorPlugin struct {
			M int64 `PluginAttribute:"m,default=111"`
			N int64 `PluginAttribute:"n,default=abc"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.N error >> parse "abc" to int64 error: strconv.ParseInt: parsing "abc": invalid syntax`)
	})

	t.Run("float64 error", func(t *testing.T) {
		type ErrorPlugin struct {
			M float64 `PluginAttribute:"m,default=111"`
			N float64 `PluginAttribute:"n,default=abc"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.N error >> parse "abc" to float64 error: strconv.ParseFloat: parsing "abc": invalid syntax`)
	})

	t.Run("boolean error", func(t *testing.T) {
		type ErrorPlugin struct {
			M bool `PluginAttribute:"m,default=true"`
			N bool `PluginAttribute:"n,default=abc"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.N error >> parse "abc" to bool error: strconv.ParseBool: parsing "abc": invalid syntax`)
	})

	t.Run("type error", func(t *testing.T) {
		type ErrorPlugin struct {
			M chan error `PluginAttribute:"m,default=true"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.M error >> unsupported inject type chan error for field at test`)
	})

	t.Run("success with name attribute", func(t *testing.T) {
		type SuccessPlugin struct {
			Name string `PluginAttribute:"name"`
		}
		typ := reflect.TypeFor[SuccessPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		v, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Nil()
		p := v.Interface().(*SuccessPlugin)
		assert.String(t, p.Name).Equal("test")
	})

	t.Run("success with default value", func(t *testing.T) {
		type SuccessPlugin struct {
			Value string `PluginAttribute:"value,default=hello"`
		}
		typ := reflect.TypeFor[SuccessPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		v, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Nil()
		p := v.Interface().(*SuccessPlugin)
		assert.String(t, p.Value).Equal("hello")
	})

	t.Run("success with storage value", func(t *testing.T) {
		type SuccessPlugin struct {
			Value string `PluginAttribute:"value"`
		}
		typ := reflect.TypeFor[SuccessPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.value", "world")
		v, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Nil()
		p := v.Interface().(*SuccessPlugin)
		assert.String(t, p.Value).Equal("world")
	})

	t.Run("success with property reference", func(t *testing.T) {
		type SuccessPlugin struct {
			Value string `PluginAttribute:"value"`
		}
		typ := reflect.TypeFor[SuccessPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("prop.value", "property_value")
		s.Set("test.value", "${prop.value}")
		v, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Nil()
		p := v.Interface().(*SuccessPlugin)
		assert.String(t, p.Value).Equal("property_value")
	})

	// Tests for array/slice injection
	t.Run("slice from comma separated value", func(t *testing.T) {
		type SlicePlugin struct {
			Values []string `PluginAttribute:"values"`
		}
		typ := reflect.TypeFor[SlicePlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.values", "apple,banana,cherry")
		v, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Nil()
		p := v.Interface().(*SlicePlugin)
		assert.Slice(t, p.Values).Equal([]string{"apple", "banana", "cherry"})
	})

	t.Run("slice from indexed keys", func(t *testing.T) {
		type SlicePlugin struct {
			Numbers []int `PluginAttribute:"numbers"`
		}
		typ := reflect.TypeFor[SlicePlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.numbers[0]", "10")
		s.Set("test.numbers[1]", "20")
		s.Set("test.numbers[2]", "30")
		v, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Nil()
		p := v.Interface().(*SlicePlugin)
		assert.Slice(t, p.Numbers).Equal([]int{10, 20, 30})
	})

	t.Run("slice from indexed keys with missing element", func(t *testing.T) {
		type SlicePlugin struct {
			Numbers []int `PluginAttribute:"numbers"`
		}
		typ := reflect.TypeFor[SlicePlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.numbers[0]", "10")
		s.Set("test.numbers[2]", "30")
		v, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field SlicePlugin.Numbers error >> missing array value test.numbers\[1\]`)
		assert.That(t, v.IsValid()).False()
	})

	t.Run("slice with property reference", func(t *testing.T) {
		type SlicePlugin struct {
			Values []string `PluginAttribute:"values"`
		}
		typ := reflect.TypeFor[SlicePlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("prop.array", "property,value,array")
		s.Set("test.values", "${prop.array}")
		v, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Nil()
		p := v.Interface().(*SlicePlugin)
		assert.Slice(t, p.Values).Equal([]string{"property", "value", "array"})
	})

	t.Run("slice conversion error", func(t *testing.T) {
		type ErrorPlugin struct {
			Numbers []int `PluginAttribute:"numbers"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.numbers", "1,abc,3")
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.Numbers error >> inject Numbers\[1] error >> parse "abc" to int error: strconv.ParseInt: parsing "abc": invalid syntax`)
	})

	t.Run("slice unsupported element type", func(t *testing.T) {
		type ErrorPlugin struct {
			Channels []chan error `PluginAttribute:"channels"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.channels", "test")
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.Channels error >> inject Channels\[0] error >> unsupported inject type chan error`)
	})
}

func TestInjectElement(t *testing.T) {

	t.Run("no element", func(t *testing.T) {
		type ErrorPlugin struct {
			Layout Layout `PluginElement:""`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.Layout error >> PluginElement tag is empty`)
	})

	t.Run("unsupported inject type", func(t *testing.T) {
		type ErrorPlugin struct {
			Layout map[string]Layout `PluginElement:"layout"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.layout.type", "TextLayout")
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.Layout error >> unsupported inject type map\[string]log.Layout`)
	})

	t.Run("no element - slice - default", func(t *testing.T) {
		type ErrorPlugin struct {
			Layouts []Layout `PluginElement:"layout"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.Layouts error >> no plugin type configured and no default specified`)
	})

	t.Run("plugin not found - slice - interface - 1", func(t *testing.T) {
		type ErrorPlugin struct {
			Layout []Layout `PluginElement:"layout,default=NotExistElement"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches("inject field ErrorPlugin.Layout error >> plugin NotExistElement not found")
	})

	t.Run("plugin not found - slice - interface - 2", func(t *testing.T) {
		type ErrorPlugin struct {
			Layout []Layout `PluginElement:"layout"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.layout.type", "NotExistElement")
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches("inject field ErrorPlugin.Layout error >> plugin NotExistElement not found")
	})

	t.Run("plugin not found - slice - struct - 1", func(t *testing.T) {
		type ErrorPlugin struct {
			AppenderRefs []*AppenderRef `PluginElement:"appenderRef"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.appenderRef[0].ref", "file")
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Nil()
	})

	t.Run("plugin not found - slice - struct - 2", func(t *testing.T) {
		type ErrorPlugin struct {
			AppenderRefs []*AppenderRef `PluginElement:"appenderRef"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.appenderRef.ref", "file")
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Nil()
	})

	t.Run("no element - single - default", func(t *testing.T) {
		type ErrorPlugin struct {
			Layouts Layout `PluginElement:"layout"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches("inject field ErrorPlugin.Layouts error >> no plugin type configured and no default specified")
	})

	t.Run("no element - single - no - type", func(t *testing.T) {
		type ErrorPlugin struct {
			Layouts Layout `PluginElement:"layout"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.layout.dummy", "")
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches("inject field ErrorPlugin.Layouts error >> no plugin type configured and no default specified")
	})

	t.Run("plugin not found - single - interface - 1", func(t *testing.T) {
		type ErrorPlugin struct {
			Layout Layout `PluginElement:"layout,default=NotExistElement"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches("inject field ErrorPlugin.Layout error >> plugin NotExistElement not found")
	})

	t.Run("plugin not found - single - interface - 2", func(t *testing.T) {
		type ErrorPlugin struct {
			Layout Layout `PluginElement:"layout"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.layout.type", "NotExistElement")
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches("inject field ErrorPlugin.Layout error >> plugin NotExistElement not found")
	})

	t.Run("plugin type mismatch - single interface", func(t *testing.T) {
		type ErrorPlugin struct {
			Layout Layout `PluginElement:"layout"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.layout.type", "FileAppender")
		s.Set("test.layout.file", "app.log")
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.Layout error >> plugin FileAppender does not implement log.Layout`)
	})

	t.Run("plugin type mismatch - slice interface", func(t *testing.T) {
		type ErrorPlugin struct {
			Layouts []Layout `PluginElement:"layout"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.layout.type", "FileAppender")
		s.Set("test.layout.file", "app.log")
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.Layouts error >> plugin FileAppender does not implement log.Layout`)
	})

	t.Run("missing indexed element - slice interface", func(t *testing.T) {
		type ErrorPlugin struct {
			Layouts []Layout `PluginElement:"layout"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.layout[1].type", "TextLayout")
		v, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.Layouts error >> missing plugin element test.layout\[0\]`)
		assert.That(t, v.IsValid()).False()
	})

	t.Run("newPlugin error - slice - 1", func(t *testing.T) {
		type ErrorPlugin struct {
			Appenders []Appender `PluginElement:"appender,default=FileAppender"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.Appenders error >> inject field FileAppender.FileName error >> no value configured and no default specified`)
	})

	t.Run("newPlugin error - slice - 2", func(t *testing.T) {
		type ErrorPlugin struct {
			Appenders []Appender `PluginElement:"appender"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		s.Set("test.appender.type", "FileAppender")
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.Appenders error >> inject field FileAppender.FileName error >> no value configured and no default specified`)
	})

	t.Run("newPlugin error - single", func(t *testing.T) {
		type ErrorPlugin struct {
			Appender Appender `PluginElement:"appender,default=FileAppender"`
		}
		typ := reflect.TypeFor[ErrorPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		_, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Matches(`inject field ErrorPlugin.Appender error >> inject field FileAppender.FileName error >> no value configured and no default specified`)
	})

	t.Run("success - slice - 1", func(t *testing.T) {
		type SuccessPlugin struct {
			Layouts []Layout `PluginElement:"layout,default=TextLayout"`
		}
		typ := reflect.TypeFor[SuccessPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		v, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Nil()
		p := v.Interface().(*SuccessPlugin)
		assert.That(t, p.Layouts).NotNil()
	})

	t.Run("success - slice -2", func(t *testing.T) {
		type SuccessPlugin struct {
			Layouts []Layout `PluginElement:"layout"`
		}
		typ := reflect.TypeFor[SuccessPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		ps.Set("test.layout.type", "TextLayout")
		v, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Nil()
		p := v.Interface().(*SuccessPlugin)
		assert.That(t, p.Layouts).NotNil()
	})

	t.Run("success - single", func(t *testing.T) {
		type SuccessPlugin struct {
			Layout Layout `PluginElement:"layout"`
		}
		typ := reflect.TypeFor[SuccessPlugin]()
		ps := flatten.NewProperties(nil)
		s := flatten.NewPropertiesStorage(ps)
		ps.Set("test.layout.type", "TextLayout")
		v, err := newPlugin(typ, "test", s)
		assert.Error(t, err).Nil()
		p := v.Interface().(*SuccessPlugin)
		assert.That(t, p.Layout).NotNil()
	})

}
