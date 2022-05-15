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

package conf_test

import (
	"errors"
	"fmt"
	"image"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-core/conf"
)

func init() {
	log.SetLevel(log.TraceLevel)
}

func TestIsValueType(t *testing.T) {

	data := []struct {
		i interface{}
		v bool
	}{
		{true, true},                 // Bool
		{int(1), true},               // Int
		{int8(1), true},              // Int8
		{int16(1), true},             // Int16
		{int32(1), true},             // Int32
		{int64(1), true},             // Int64
		{uint(1), true},              // Uint
		{uint8(1), true},             // Uint8
		{uint16(1), true},            // Uint16
		{uint32(1), true},            // Uint32
		{uint64(1), true},            // Uint64
		{uintptr(0), false},          // Uintptr
		{float32(1), true},           // Float32
		{float64(1), true},           // Float64
		{complex64(1), true},         // Complex64
		{complex128(1), true},        // Complex128
		{[1]int{0}, true},            // Array
		{make(chan struct{}), false}, // Chan
		{func() {}, false},           // Func
		{reflect.TypeOf((*error)(nil)).Elem(), false}, // Interface
		{make(map[int]int), true},                     // Map
		{make(map[string]*int), false},                //
		{new(int), false},                             // Ptr
		{new(struct{}), false},                        //
		{[]int{0}, true},                              // Slice
		{[]*int{}, false},                             //
		{"this is a string", true},                    // String
		{struct{}{}, true},                            // Struct
		{unsafe.Pointer(new(int)), false},             // UnsafePointer
	}

	for _, d := range data {
		var typ reflect.Type
		switch i := d.i.(type) {
		case reflect.Type:
			typ = i
		default:
			typ = reflect.TypeOf(i)
		}
		if r := conf.IsValueType(typ); d.v != r {
			t.Errorf("%v expect %v but %v", typ, d.v, r)
		}
	}
}

func TestProperties_Load(t *testing.T) {

	p := conf.New()
	err := p.Load("testdata/config/application.yaml")
	assert.Nil(t, err)
	err = p.Load("testdata/config/application.properties")
	assert.Nil(t, err)

	for _, k := range p.Keys() {
		fmt.Println(k+":", p.Get(k))
	}

	assert.True(t, p.Has("yaml.list"))
	assert.True(t, p.Has("properties.list"))
}

func TestProperties_ReadProperties(t *testing.T) {

	t.Run("basic type", func(t *testing.T) {

		data := []struct {
			key  string
			str  string
			val  interface{}
			kind reflect.Kind
		}{
			{"bool", "bool=false", "false", reflect.String},
			{"int", "int=3", "3", reflect.String},
			{"float", "float=3.0", "3.0", reflect.String},
			{"string", "string=3", "3", reflect.String},
			{"string", "string=hello", "hello", reflect.String},
			{"date", "date=2018-02-17", "2018-02-17", reflect.String},
			{"time", "time=2018-02-17T15:02:31+08:00", "2018-02-17T15:02:31+08:00", reflect.String},
		}

		for _, d := range data {
			p, _ := conf.Bytes([]byte(d.str), ".properties")
			v := p.Get(d.key)
			assert.Equal(t, v, d.val)
		}
	})

	t.Run("map", func(t *testing.T) {

		str := `
          map.bool=false
          map.int=3
          map.float=3.0
          map.string=hello
        `

		data := map[string]interface{}{
			"map.bool":   "false",
			"map.float":  "3.0",
			"map.int":    "3",
			"map.string": "hello",
		}

		p, _ := conf.Bytes([]byte(str), ".properties")
		for k, expect := range data {
			v := p.Get(k)
			assert.Equal(t, v, expect)
		}
	})

	t.Run("array struct", func(t *testing.T) {

		str := `
	         array[0].bool=false
	         array[0].int=3
	         array[0].float=3.0
	         array[0].string=hello
	         array[1].bool=true
	         array[1].int=20
	         array[1].float=0.2
	         array[1].string=hello
	       `

		p, _ := conf.Bytes([]byte(str), ".properties")
		data := map[string]interface{}{
			"array[0].bool":   "false",
			"array[0].int":    "3",
			"array[0].float":  "3.0",
			"array[0].string": "hello",
			"array[1].bool":   "true",
			"array[1].int":    "20",
			"array[1].float":  "0.2",
			"array[1].string": "hello",
		}

		for k, expect := range data {
			v := p.Get(k)
			assert.Equal(t, v, expect)
		}
	})

	t.Run("map struct", func(t *testing.T) {

		str := `
          map.k1.bool: false
          map.k1.int: 3
          map.k1.float: 3.0
          map.k1.string: hello
          map.k2.bool: true
          map.k2.int: 20
          map.k2.float: 0.2
          map.k2.string: hello
        `

		data := map[string]interface{}{
			"map.k1.bool":   "false",
			"map.k1.float":  "3.0",
			"map.k1.int":    "3",
			"map.k1.string": "hello",
			"map.k2.bool":   "true",
			"map.k2.float":  "0.2",
			"map.k2.int":    "20",
			"map.k2.string": "hello",
		}

		p, _ := conf.Bytes([]byte(str), ".properties")
		for k, expect := range data {
			v := p.Get(k)
			assert.Equal(t, v, expect)
		}
	})
}

func TestProperties_ReadYaml(t *testing.T) {

	t.Run("basic type", func(t *testing.T) {

		data := []struct {
			key  string
			str  string
			val  interface{}
			kind reflect.Kind
		}{
			{"bool", "bool: false", "false", reflect.Bool},
			{"int", "int: 3", "3", reflect.Int},
			{"float", "float: 3.0", "3", reflect.Float64},
			{"string", "string: \"3\"", "3", reflect.String},
			{"string", "string: hello", "hello", reflect.String},
			{"date", "date: 2018-02-17", "2018-02-17", reflect.String},
			{"time", "time: 2018-02-17T15:02:31+08:00", "2018-02-17T15:02:31+08:00", reflect.String},
		}

		for _, d := range data {
			p, _ := conf.Bytes([]byte(d.str), ".yaml")
			v := p.Get(d.key)
			assert.Equal(t, v, d.val)
		}
	})

	t.Run("map", func(t *testing.T) {

		str := `
          map: 
              bool: false
              int: 3
              float: 3.0
              string: hello
        `

		data := map[string]interface{}{
			"map.bool":   "false",
			"map.float":  "3",
			"map.int":    "3",
			"map.string": "hello",
		}

		p, _ := conf.Bytes([]byte(str), ".yaml")
		for k, expect := range data {
			v := p.Get(k)
			assert.Equal(t, v, expect)
		}
	})

	t.Run("array struct", func(t *testing.T) {

		str := `
          array: 
              -
                  bool: false
                  int: 3
                  float: 3.0
                  string: hello
              -
                  bool: true
                  int: 20
                  float: 0.2
                  string: hello
        `

		p, _ := conf.Bytes([]byte(str), ".yaml")

		data := []struct {
			key  string
			val  interface{}
			kind reflect.Kind
		}{
			{"array[0].bool", "false", reflect.Bool},
			{"array[0].int", "3", reflect.Int},
			{"array[0].float", "3", reflect.Float64},
			{"array[0].string", "hello", reflect.String},
			{"array[1].bool", "true", reflect.Bool},
			{"array[1].int", "20", reflect.Int},
			{"array[1].float", "0.2", reflect.Float64},
			{"array[1].string", "hello", reflect.String},
		}

		for _, d := range data {
			v := p.Get(d.key)
			assert.Equal(t, v, d.val)
		}
	})

	t.Run("map struct", func(t *testing.T) {

		str := `
          map: 
              k1: 
                  bool: false
                  int: 3
                  float: 3.0
                  string: hello
              k2: 
                  bool: true
                  int: 20
                  float: 0.2
                  string: hello
        `

		p, _ := conf.Bytes([]byte(str), ".yaml")

		data := []struct {
			key  string
			val  interface{}
			kind reflect.Kind
		}{
			{"map.k1.bool", "false", reflect.Bool},
			{"map.k1.int", "3", reflect.Int},
			{"map.k1.float", "3", reflect.Float64},
			{"map.k1.string", "hello", reflect.String},
			{"map.k2.bool", "true", reflect.Bool},
			{"map.k2.int", "20", reflect.Int},
			{"map.k2.float", "0.2", reflect.Float64},
			{"map.k2.string", "hello", reflect.String},
		}

		for _, d := range data {
			v := p.Get(d.key)
			assert.Equal(t, v, d.val)
		}
	})

	t.Run("", func(t *testing.T) {
		p, err := conf.Bytes([]byte("array: []\nmap: {}\n"), ".yaml")
		assert.Nil(t, err)
		assert.True(t, p.Has("map"))
		assert.True(t, p.Has("array"))
	})
}

func TestProperties_ReadToml(t *testing.T) {

	t.Run("basic type", func(t *testing.T) {

		data := []struct {
			key  string
			str  string
			val  interface{}
			kind reflect.Kind
		}{
			{"bool", "bool=false", "false", reflect.Bool},
			{"int", "int=3", "3", reflect.Int},
			{"float", "float=3.0", "3", reflect.Float64},
			{"string", "string=\"3\"", "3", reflect.String},
			{"string", "string=\"hello\"", "hello", reflect.String},
			{"date", "date=\"2018-02-17\"", "2018-02-17", reflect.String},
			{"time", "time=\"2018-02-17T15:02:31+08:00\"", "2018-02-17T15:02:31+08:00", reflect.String},
		}

		for _, d := range data {
			p, _ := conf.Bytes([]byte(d.str), ".toml")
			v := p.Get(d.key)
			assert.Equal(t, v, d.val)
		}
	})

	t.Run("map", func(t *testing.T) {

		str := `
          [map]
          bool=false
          int=3
          float=3.0
          string="hello"
        `

		data := map[string]interface{}{
			"map.bool":   "false",
			"map.float":  "3",
			"map.int":    "3",
			"map.string": "hello",
		}

		p, _ := conf.Bytes([]byte(str), ".toml")
		for k, expect := range data {
			v := p.Get(k)
			assert.Equal(t, v, expect)
		}
	})

	t.Run("array struct", func(t *testing.T) {

		str := `
          [[array]]
          bool=false
          int=3
          float=3.0
          string="hello"

          [[array]]
          bool=true
          int=20
          float=0.2
          string="hello"
        `

		p, _ := conf.Bytes([]byte(str), ".toml")

		data := []struct {
			key  string
			val  interface{}
			kind reflect.Kind
		}{
			{"array[0].bool", "false", reflect.Bool},
			{"array[0].int", "3", reflect.Int},
			{"array[0].float", "3", reflect.Float64},
			{"array[0].string", "hello", reflect.String},
			{"array[1].bool", "true", reflect.Bool},
			{"array[1].int", "20", reflect.Int},
			{"array[1].float", "0.2", reflect.Float64},
			{"array[1].string", "hello", reflect.String},
		}

		for _, d := range data {
			v := p.Get(d.key)
			assert.Equal(t, v, d.val)
		}
	})

	t.Run("map struct", func(t *testing.T) {

		str := `
          [map.k1]
          bool=false
          int=3
          float=3.0
          string="hello"
          
          [map.k2]
          bool=true
          int=20
          float=0.2
          string="hello"
        `

		p, _ := conf.Bytes([]byte(str), ".toml")

		data := []struct {
			key  string
			val  interface{}
			kind reflect.Kind
		}{
			{"map.k1.bool", "false", reflect.Bool},
			{"map.k1.int", "3", reflect.Int},
			{"map.k1.float", "3", reflect.Float64},
			{"map.k1.string", "hello", reflect.String},
			{"map.k2.bool", "true", reflect.Bool},
			{"map.k2.int", "20", reflect.Int},
			{"map.k2.float", "0.2", reflect.Float64},
			{"map.k2.string", "hello", reflect.String},
		}

		for _, d := range data {
			v := p.Get(d.key)
			assert.Equal(t, v, d.val)
		}
	})
}

func TestProperties_Get(t *testing.T) {

	t.Run("base", func(t *testing.T) {

		p := conf.New()

		err := p.Set("a.b.c", "3")
		assert.Nil(t, err)
		err = p.Set("a.b.d", []string{"3"})
		assert.Nil(t, err)

		v := p.Get("a.b.c")
		assert.Equal(t, v, "3")
		v = p.Get("a.b.d[0]")
		assert.Equal(t, v, "3")

		err = p.Set("Bool", true)
		assert.Nil(t, err)
		err = p.Set("Int", 3)
		assert.Nil(t, err)
		err = p.Set("Uint", 3)
		assert.Nil(t, err)
		err = p.Set("Float", 3.0)
		assert.Nil(t, err)
		err = p.Set("String", "3")
		assert.Nil(t, err)
		err = p.Set("Duration", "3s")
		assert.Nil(t, err)
		err = p.Set("StringSlice", []string{"3", "4"})
		assert.Nil(t, err)
		err = p.Set("Time", "2020-02-04 20:02:04 >> 2006-01-02 15:04:05")
		assert.Nil(t, err)
		err = p.Set("MapStringInterface", []interface{}{
			map[interface{}]interface{}{
				"1": 2,
			},
		})
		assert.Nil(t, err)

		assert.False(t, p.Has("NULL"))
		assert.Equal(t, p.Get("NULL"), "")

		v = p.Get("NULL", conf.Def("OK"))
		assert.Equal(t, v, "OK")

		v = p.Get("Int")
		assert.Equal(t, v, "3")

		var v2 int
		err = p.Bind(&v2, conf.Key("Int"))
		assert.Nil(t, err)
		assert.Equal(t, v2, 3)

		var u2 uint
		err = p.Bind(&u2, conf.Key("Uint"))
		assert.Nil(t, err)
		assert.Equal(t, u2, uint(3))

		var u3 uint32
		err = p.Bind(&u3, conf.Key("Uint"))
		assert.Nil(t, err)
		assert.Equal(t, u3, uint32(3))

		var f2 float32
		err = p.Bind(&f2, conf.Key("Float"))
		assert.Nil(t, err)
		assert.Equal(t, f2, float32(3))

		v = p.Get("Bool")
		b := cast.ToBool(v)
		assert.Equal(t, b, true)

		var b2 bool
		err = p.Bind(&b2, conf.Key("Bool"))
		assert.Nil(t, err)
		assert.Equal(t, b2, true)

		v = p.Get("Int")
		i := cast.ToInt64(v)
		assert.Equal(t, i, int64(3))

		v = p.Get("Uint")
		u := cast.ToUint64(v)
		assert.Equal(t, u, uint64(3))

		v = p.Get("Float")
		f := cast.ToFloat64(v)
		assert.Equal(t, f, 3.0)

		v = p.Get("String")
		s := cast.ToString(v)
		assert.Equal(t, s, "3")

		assert.False(t, p.Has("string"))
		assert.Equal(t, p.Get("string"), "")

		v = p.Get("Duration")
		d := cast.ToDuration(v)
		assert.Equal(t, d, time.Second*3)

		var ti time.Time
		err = p.Bind(&ti, conf.Key("Time"))
		assert.Nil(t, err)
		assert.Equal(t, ti, time.Date(2020, 02, 04, 20, 02, 04, 0, time.UTC))

		err = p.Bind(&ti, conf.Key("Duration"))
		assert.Nil(t, err)
		assert.Equal(t, ti, time.Date(1970, 01, 01, 00, 00, 03, 0, time.UTC).Local())

		var ss2 []string
		err = p.Bind(&ss2, conf.Key("StringSlice"))
		assert.Nil(t, err)
		assert.Equal(t, ss2, []string{"3", "4"})
	})

	t.Run("slice slice", func(t *testing.T) {
		p, err := conf.Map(map[string]interface{}{
			"a": []interface{}{
				[]interface{}{1, 2},
				[]interface{}{3, 4},
				map[string]interface{}{
					"b": "c",
					"d": []interface{}{5, 6},
				},
			},
		})
		assert.Nil(t, err)
		v := p.Get("a[0][0]")
		assert.Equal(t, v, "1")
		v = p.Get("a[0][1]")
		assert.Equal(t, v, "2")
		v = p.Get("a[1][0]")
		assert.Equal(t, v, "3")
		v = p.Get("a[1][1]")
		assert.Equal(t, v, "4")
		v = p.Get("a[2].b")
		assert.Equal(t, v, "c")
		v = p.Get("a[2].d[0]")
		assert.Equal(t, v, "5")
		v = p.Get("a[2].d[1]")
		assert.Equal(t, v, "6")
	})
}

func TestProperties_Ref(t *testing.T) {

	type fileLog struct {
		Dir             string `value:"${dir:=${app.dir}}"`
		NestedDir       string `value:"${nested.dir:=${nested.app.dir:=./log}}"`
		NestedEmptyDir  string `value:"${nested.dir:=${nested.app.dir:=}}"`
		NestedNestedDir string `value:"${nested.dir:=${nested.app.dir:=${nested.nested.app.dir:=./log}}}"`
	}

	var mqLog struct{ fileLog }
	var httpLog struct{ fileLog }

	t.Run("not config", func(t *testing.T) {
		p := conf.New()
		err := p.Bind(&httpLog)
		assert.Error(t, err, "property \\\"app.dir\\\" not exist")
	})

	t.Run("config", func(t *testing.T) {
		p := conf.New()

		appDir := "/home/log"
		err := p.Set("app.dir", appDir)
		assert.Nil(t, err)

		err = p.Bind(&httpLog)
		assert.Nil(t, err)
		assert.Equal(t, httpLog.Dir, appDir)
		assert.Equal(t, httpLog.NestedDir, "./log")
		assert.Equal(t, httpLog.NestedEmptyDir, "")
		assert.Equal(t, httpLog.NestedNestedDir, "./log")

		err = p.Bind(&mqLog)
		assert.Nil(t, err)
		assert.Equal(t, mqLog.Dir, appDir)
		assert.Equal(t, mqLog.NestedDir, "./log")
		assert.Equal(t, mqLog.NestedEmptyDir, "")
		assert.Equal(t, mqLog.NestedNestedDir, "./log")
	})

	t.Run("empty key", func(t *testing.T) {
		p := conf.New()
		var s struct {
			KeyIsEmpty string `value:"${:=kie}"`
		}
		err := p.Bind(&s)
		assert.Nil(t, err)
		assert.Equal(t, s.KeyIsEmpty, "kie")
	})
}

func TestBindMap(t *testing.T) {

	t.Run("", func(t *testing.T) {
		var r [3]map[string]string
		err := conf.New().Bind(&r)
		assert.Error(t, err, ".*bind.go:.* bind \\[3]map\\[string]string error; target should be value type")
	})

	t.Run("", func(t *testing.T) {
		var r []map[string]string
		err := conf.New().Bind(&r)
		assert.Error(t, err, ".*bind.go:.* bind \\[]map\\[string]string error; target should be value type")
	})

	t.Run("", func(t *testing.T) {
		var r map[string]map[string]string
		err := conf.New().Bind(&r)
		assert.Error(t, err, ".*bind.go:.* bind map\\[string]map\\[string]string error; target should be value type")
	})

	m := map[string]interface{}{
		"a.b1": "ab1",
		"a.b2": "ab2",
		"a.b3": "ab3",
		"b.b1": "bb1",
		"b.b2": "bb2",
		"b.b3": "bb3",
	}

	t.Run("", func(t *testing.T) {
		type S struct {
			M [3]map[string]string `value:"${}"`
		}
		var r map[string]S
		p, err := conf.Map(m)
		assert.Nil(t, err)
		err = p.Bind(&r)
		assert.Error(t, err, ".*bind.go:.* bind map\\[string]conf_test.S.M error; target should be value type")
	})

	t.Run("", func(t *testing.T) {
		type S struct {
			M []map[string]string `value:"${}"`
		}
		var r map[string]S
		p, err := conf.Map(m)
		assert.Nil(t, err)
		err = p.Bind(&r)
		assert.Error(t, err, ".*bind.go:.* bind map\\[string]conf_test.S.M error; target should be value type")
	})

	t.Run("", func(t *testing.T) {
		type S struct {
			M map[string]map[string]string `value:"${}"`
		}
		var r map[string]S
		p, err := conf.Map(m)
		assert.Nil(t, err)
		err = p.Bind(&r)
		assert.Error(t, err, ".*bind.go:.* bind map\\[string]conf_test.S.M error; target should be value type")
	})

	t.Run("", func(t *testing.T) {
		var r map[string]struct {
			B1 string `value:"${b1}"`
			B2 string `value:"${b2}"`
			B3 string `value:"${b3}"`
		}
		p, err := conf.Map(m)
		assert.Nil(t, err)
		err = p.Bind(&r)
		assert.Nil(t, err)
		assert.Equal(t, r["a"].B1, "ab1")
	})

	t.Run("", func(t *testing.T) {
		p, err := conf.Map(map[string]interface{}{"a.b1": "ab1"})
		assert.Nil(t, err)
		var r map[string]string
		err = p.Bind(&r)
		assert.Error(t, err, ".*bind.go:.* bind map\\[string]string error; .*bind.go:.* resolve property \"a\" error; property \"a\" not exist")
	})

	t.Run("", func(t *testing.T) {
		type S struct {
			A map[string]string `value:"${a}"`
			B map[string]string `value:"${b}"`
		}
		var r S
		p, err := conf.Map(map[string]interface{}{
			"a": "1", "b": 2,
		})
		assert.Nil(t, err)
		err = p.Bind(&r)
		assert.Error(t, err, ".*bind.go:.* bind S error; .*bind.go:.* bind S.A error; property \"a\" isn't map")
	})

	t.Run("", func(t *testing.T) {
		var r struct {
			A map[string]string `value:"${a}"`
			B map[string]string `value:"${b}"`
		}
		p, err := conf.Map(m)
		assert.Nil(t, err)
		err = p.Bind(&r)
		assert.Nil(t, err)
		assert.Equal(t, r.A["b1"], "ab1")
	})
}

func TestResolve(t *testing.T) {
	p := conf.New()
	err := p.Set("name", "Jim")
	assert.Nil(t, err)
	_, err = p.Resolve("my name is ${name")
	assert.Error(t, err, ".*bind.go:.* resolve string \"my name is \\${name\" error; invalid syntax")
	str, _ := p.Resolve("my name is ${name}")
	assert.Equal(t, str, "my name is Jim")
	str, _ = p.Resolve("my name is ${name}${name}")
	assert.Equal(t, str, "my name is JimJim")
	_, err = p.Resolve("my name is ${name} my name is ${name")
	assert.NotNil(t, err)
	str, _ = p.Resolve("my name is ${name} my name is ${name}")
	assert.Equal(t, str, "my name is Jim my name is Jim")
}

func TestProperties_Has(t *testing.T) {
	p, err := conf.Map(map[string]interface{}{
		"a.b.c": "3",
		"a.b.d": []string{"7", "8"},
	})
	assert.Nil(t, err)
	assert.True(t, p.Has("a"))
	assert.False(t, p.Has("a[0]"))
	assert.True(t, p.Has("a.b"))
	assert.False(t, p.Has("a.c"))
	assert.True(t, p.Has("a.b.c"))
	assert.True(t, p.Has("a.b.d"))
	assert.True(t, p.Has("a.b.d[0]"))
	assert.True(t, p.Has("a.b.d[1]"))
	assert.False(t, p.Has("a.b.d[2]"))
	assert.False(t, p.Has("a.b.e"))
}

func TestProperties_Set(t *testing.T) {

	t.Run("", func(t *testing.T) {
		p := conf.New()
		err := p.Set("a", []string{})
		assert.Nil(t, err)
		val := p.Get("a")
		assert.Equal(t, val, "")
	})

	p := conf.New()
	err := p.Set("a", []string{"a", "aa", "aaa"})
	assert.Nil(t, err)
	err = p.Set("b", []int{1, 11, 111})
	assert.Nil(t, err)
	err = p.Set("c", []float32{1, 1.1, 1.11})
	assert.Nil(t, err)
	assert.Equal(t, p.Get("a[0]"), "a")
	assert.Equal(t, p.Get("a[1]"), "aa")
	assert.Equal(t, p.Get("a[2]"), "aaa")
	assert.Equal(t, p.Get("b[0]"), "1")
	assert.Equal(t, p.Get("b[1]"), "11")
	assert.Equal(t, p.Get("b[2]"), "111")
	assert.Equal(t, p.Get("c[0]"), "1")
	assert.Equal(t, p.Get("c[1]"), "1.1")
	assert.Equal(t, p.Get("c[2]"), "1.11")
}

func PointConverter(val string) (image.Point, error) {
	if !(strings.HasPrefix(val, "(") && strings.HasSuffix(val, ")")) {
		return image.Point{}, errors.New("数据格式错误")
	}
	ss := strings.Split(val[1:len(val)-1], ",")
	x := cast.ToInt(ss[0])
	y := cast.ToInt(ss[1])
	return image.Point{X: x, Y: y}, nil
}

func PointSplitter(str string) ([]string, error) {
	var ret []string
	var lastIndex int
	for i, c := range str {
		if c == ')' {
			ret = append(ret, str[lastIndex:i+1])
			lastIndex = i + 1
		}
	}
	return ret, nil
}

func TestSplitter(t *testing.T) {
	conf.RegisterConverter(PointConverter)
	conf.RegisterSplitter("PointSplitter", PointSplitter)
	var points []image.Point
	err := conf.New().Bind(&points, conf.Tag("${:=(1,2)(3,4)}||PointSplitter"))
	assert.Nil(t, err)
	assert.Equal(t, points, []image.Point{{X: 1, Y: 2}, {X: 3, Y: 4}})
}

func TestSplitPath(t *testing.T) {
	var testcases = []struct {
		Key  string
		Err  error
		Path []string
	}{
		{
			Key: "",
			Err: errors.New("error key ''"),
		},
		{
			Key: " ",
			Err: errors.New("error key ' '"),
		},
		{
			Key: ".",
			Err: errors.New("error key '.'"),
		},
		{
			Key: "[",
			Err: errors.New("error key '['"),
		},
		{
			Key: "]",
			Err: errors.New("error key ']'"),
		},
		{
			Key: "[]",
			Err: errors.New("error key '[]'"),
		},
		{
			Key:  "[0]",
			Path: []string{"0"},
		},
		{
			Key: "[0][",
			Err: errors.New("error key '[0]['"),
		},
		{
			Key: "[0]]",
			Err: errors.New("error key '[0]]'"),
		},
		{
			Key: "[[0]]",
			Err: errors.New("error key '[[0]]'"),
		},
		{
			Key: "[.]",
			Err: errors.New("error key '[.]'"),
		},
		{
			Key: "[a.b]",
			Err: errors.New("error key '[a.b]'"),
		},
		{
			Key:  "a",
			Path: []string{"a"},
		},
		{
			Key: "a.",
			Err: errors.New("error key 'a.'"),
		},
		{
			Key:  "a.b",
			Path: []string{"a", "b"},
		},
		{
			Key: "a[",
			Err: errors.New("error key 'a['"),
		},
		{
			Key: "a]",
			Err: errors.New("error key 'a]'"),
		},
		{
			Key:  "a[0]",
			Path: []string{"a", "0"},
		},
		{
			Key:  "a.[0]",
			Path: []string{"a", "0"},
		},
		{
			Key:  "a[0].b",
			Path: []string{"a", "0", "b"},
		},
		{
			Key:  "a.[0].b",
			Path: []string{"a", "0", "b"},
		},
		{
			Key:  "a[0][0]",
			Path: []string{"a", "0", "0"},
		},
		{
			Key:  "a.[0].[0]",
			Path: []string{"a", "0", "0"},
		},
	}
	for i, c := range testcases {
		p, err := conf.SplitPath(c.Key)
		assert.Equal(t, err, c.Err, fmt.Sprintf("index: %d key: %q", i, c.Key))
		assert.Equal(t, p, c.Path, fmt.Sprintf("index:%d key: %q", i, c.Key))
	}
}
