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

package SpringCore_test

import (
	"errors"
	"fmt"
	"image"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-spring/go-spring/spring-core"
	"github.com/magiconair/properties/assert"
	"github.com/spf13/cast"
)

func TestDefaultProperties_LoadProperties(t *testing.T) {

	p := SpringCore.NewDefaultProperties()
	p.LoadProperties("testdata/config/application.yaml")
	p.LoadProperties("testdata/config/application.properties")

	fmt.Println(">>> Get All Properties")
	for k, v := range p.GetProperties() {
		fmt.Println(k, v)
	}
}

func TestDefaultProperties_ReadProperties_Properties(t *testing.T) {

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
			{"string", "string=\"3\"", "\"3\"", reflect.String},
			{"string", "string=hello", "hello", reflect.String},
			{"date", "date=2018-02-17", "2018-02-17", reflect.String},
			{"time", "time=2018-02-17T15:02:31+08:00", "2018-02-17T15:02:31+08:00", reflect.String},
		}

		for _, d := range data {
			p := SpringCore.NewDefaultProperties()
			r := strings.NewReader(d.str)
			p.ReadProperties(r, "properties")
			v := p.GetProperty(d.key)
			assert.Equal(t, v, d.val)
		}
	})

	t.Run("array", func(t *testing.T) {

		data := []struct {
			key  string
			str  string
			val  interface{}
			kind reflect.Kind
		}{
			{"bool[0]", "bool[0]=false", "false", reflect.String},
			{"int[0]", "int[0]=3", "3", reflect.String},
			{"float[0]", "float[0]=3.0", "3.0", reflect.String},
			{"string[0]", "string[0]=\"3\"", "\"3\"", reflect.String},
			{"string[0]", "string[0]=hello", "hello", reflect.String},
		}

		for _, d := range data {
			p := SpringCore.NewDefaultProperties()
			r := strings.NewReader(d.str)
			p.ReadProperties(r, "properties")
			v := p.GetProperty(d.key)
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

		p := SpringCore.NewDefaultProperties()
		r := strings.NewReader(str)
		p.ReadProperties(r, "properties")

		for k, expect := range data {
			v := p.GetProperty(k)
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

		p := SpringCore.NewDefaultProperties()
		r := strings.NewReader(str)
		p.ReadProperties(r, "properties")

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
			v := p.GetProperty(k)
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

		p := SpringCore.NewDefaultProperties()
		r := strings.NewReader(str)
		p.ReadProperties(r, "properties")

		for k, expect := range data {
			v := p.GetProperty(k)
			assert.Equal(t, v, expect)
		}
	})
}

func TestDefaultProperties_ReadProperties_Yaml(t *testing.T) {

	t.Run("basic type", func(t *testing.T) {

		data := []struct {
			key  string
			str  string
			val  interface{}
			kind reflect.Kind
		}{
			{"bool", "bool: false", false, reflect.Bool},
			{"int", "int: 3", int(3), reflect.Int}, // yaml 是 int，toml 是 int64。
			{"float", "float: 3.0", float64(3.0), reflect.Float64},
			{"string", "string: \"3\"", "3", reflect.String},
			{"string", "string: hello", "hello", reflect.String},
			{"date", "date: 2018-02-17", "2018-02-17", reflect.String},
			{"time", "time: 2018-02-17T15:02:31+08:00", "2018-02-17T15:02:31+08:00", reflect.String},
		}

		for _, d := range data {
			p := SpringCore.NewDefaultProperties()
			r := strings.NewReader(d.str)
			p.ReadProperties(r, "yaml")
			v := p.GetProperty(d.key)
			assert.Equal(t, v, d.val)
		}
	})

	t.Run("array", func(t *testing.T) {

		data := []struct {
			key  string
			str  string
			val  interface{}
			kind reflect.Kind
		}{
			{"bool", "bool: [false]", []interface{}{false}, reflect.Bool},
			{"int", "int: [3]", []interface{}{3}, reflect.Int},
			{"float", "float: [3.0]", []interface{}{3.0}, reflect.Float64},
			{"string", "string: [\"3\"]", []interface{}{"3"}, reflect.String},
			{"string", "string: [hello]", []interface{}{"hello"}, reflect.String},
		}

		for _, d := range data {
			p := SpringCore.NewDefaultProperties()
			r := strings.NewReader(d.str)
			p.ReadProperties(r, "yaml")
			v := p.GetProperty(d.key)
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
			"map.bool":   false,
			"map.float":  3.0,
			"map.int":    3,
			"map.string": "hello",
		}

		p := SpringCore.NewDefaultProperties()
		r := strings.NewReader(str)
		p.ReadProperties(r, "yaml")

		for k, expect := range data {
			v := p.GetProperty(k)
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

		p := SpringCore.NewDefaultProperties()
		r := strings.NewReader(str)
		p.ReadProperties(r, "yaml")

		v := p.GetProperty("array")
		expect := []interface{}{
			map[interface{}]interface{}{ // yaml 是 map[interface{}]interface{}，toml 是 map[string]interface{}
				"bool":   false,
				"int":    3,
				"float":  3.0,
				"string": "hello",
			},
			map[interface{}]interface{}{
				"bool":   true,
				"int":    20,
				"float":  0.2,
				"string": "hello",
			},
		}
		assert.Equal(t, v, expect)
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

		data := map[string]interface{}{
			"map.k1.bool":   false,
			"map.k1.float":  3.0,
			"map.k1.int":    3,
			"map.k1.string": "hello",
			"map.k2.bool":   true,
			"map.k2.float":  0.2,
			"map.k2.int":    20,
			"map.k2.string": "hello",
		}

		p := SpringCore.NewDefaultProperties()
		r := strings.NewReader(str)
		p.ReadProperties(r, "yaml")

		for k, expect := range data {
			v := p.GetProperty(k)
			assert.Equal(t, v, expect)
		}
	})
}

func TestDefaultProperties_ReadProperties_Toml(t *testing.T) {

	t.Run("basic type", func(t *testing.T) {

		data := []struct {
			key  string
			str  string
			val  interface{}
			kind reflect.Kind
		}{
			{"bool", "bool=false", false, reflect.Bool},
			{"int", "int=3", int64(3), reflect.Int64}, // yaml 是 int，toml 是 int64。
			{"float", "float=3.0", 3.0, reflect.Float64},
			{"string", "string=\"3\"", "3", reflect.String},
			{"string", "string=\"hello\"", "hello", reflect.String},
			{"date", "date=\"2018-02-17\"", "2018-02-17", reflect.String},
			{"time", "time=\"2018-02-17T15:02:31+08:00\"", "2018-02-17T15:02:31+08:00", reflect.String},
		}

		for _, d := range data {
			p := SpringCore.NewDefaultProperties()
			r := strings.NewReader(d.str)
			p.ReadProperties(r, "toml")
			v := p.GetProperty(d.key)
			assert.Equal(t, v, d.val)
		}
	})

	t.Run("array", func(t *testing.T) {

		data := []struct {
			key  string
			str  string
			val  interface{}
			kind reflect.Kind
		}{
			{"bool", "bool=[false]", []interface{}{false}, reflect.Bool},
			{"int", "int=[3]", []interface{}{int64(3)}, reflect.Int},
			{"float", "float=[3.0]", []interface{}{3.0}, reflect.Float64},
			{"string", "string=[\"3\"]", []interface{}{"3"}, reflect.String},
			{"string", "string=[\"hello\"]", []interface{}{"hello"}, reflect.String},
		}

		for _, d := range data {
			p := SpringCore.NewDefaultProperties()
			r := strings.NewReader(d.str)
			p.ReadProperties(r, "toml")
			v := p.GetProperty(d.key)
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
			"map.bool":   false,
			"map.float":  3.0,
			"map.int":    int64(3),
			"map.string": "hello",
		}

		p := SpringCore.NewDefaultProperties()
		r := strings.NewReader(str)
		p.ReadProperties(r, "toml")

		for k, expect := range data {
			v := p.GetProperty(k)
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

		p := SpringCore.NewDefaultProperties()
		r := strings.NewReader(str)
		p.ReadProperties(r, "toml")

		v := p.GetProperty("array")
		expect := []interface{}{
			map[string]interface{}{ // yaml 是 map[interface{}]interface{}，toml 是 map[string]interface{}
				"bool":   false,
				"int":    int64(3),
				"float":  float64(3.0),
				"string": "hello",
			},
			map[string]interface{}{
				"bool":   true,
				"int":    int64(20),
				"float":  float64(0.2),
				"string": "hello",
			},
		}
		assert.Equal(t, v, expect)
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

		data := map[string]interface{}{
			"map.k1.bool":   false,
			"map.k1.float":  float64(3.0),
			"map.k1.int":    int64(3),
			"map.k1.string": "hello",
			"map.k2.bool":   true,
			"map.k2.float":  float64(0.2),
			"map.k2.int":    int64(20),
			"map.k2.string": "hello",
		}

		p := SpringCore.NewDefaultProperties()
		r := strings.NewReader(str)
		p.ReadProperties(r, "toml")

		for k, expect := range data {
			v := p.GetProperty(k)
			assert.Equal(t, v, expect)
		}
	})
}

func PointConverter(val string) image.Point {
	if !(strings.HasPrefix(val, "(") && strings.HasSuffix(val, ")")) {
		panic(errors.New("数据格式错误"))
	}
	ss := strings.Split(val[1:len(val)-1], ",")
	x := cast.ToInt(ss[0])
	y := cast.ToInt(ss[1])
	return image.Point{X: x, Y: y}
}

func TestRegisterTypeConverter(t *testing.T) {

	assert.Panic(t, func() { // 不是函数
		SpringCore.RegisterTypeConverter(3)
	}, "fn must be func\\(string\\)type")

	assert.Panic(t, func() { // 入参太多
		SpringCore.RegisterTypeConverter(func(_ string, _ string) image.Point {
			return image.Point{}
		})
	}, "fn must be func\\(string\\)type")

	assert.Panic(t, func() { // 返回值太多
		SpringCore.RegisterTypeConverter(func(_ string) (image.Point, image.Point) {
			return image.Point{}, image.Point{}
		})
	}, "fn must be func\\(string\\)type")

	SpringCore.RegisterTypeConverter(PointConverter)
}

func TestDefaultProperties_GetProperty(t *testing.T) {
	p := SpringCore.NewDefaultProperties()

	p.SetProperty("a.b.c", "3")
	p.SetProperty("a.b.d", []string{"3"})

	m := p.GetPrefixProperties("a.b")
	assert.Equal(t, len(m), 2)
	assert.Equal(t, m["a.b.c"], "3")
	assert.Equal(t, m["a.b.d"], []string{"3"})

	p.SetProperty("Bool", true)
	p.SetProperty("Int", 3)
	p.SetProperty("Uint", 3)
	p.SetProperty("Float", 3.0)
	p.SetProperty("String", "3")
	p.SetProperty("Duration", "3s")
	p.SetProperty("[]String", []string{"3"})
	p.SetProperty("Time", "2020-02-04 20:02:04")
	p.SetProperty("[]Map[String]Interface{}", []interface{}{
		map[interface{}]interface{}{
			"1": 2,
		},
	})

	v := p.GetProperty("NULL")
	assert.Equal(t, v, nil)

	v, ok := p.GetDefaultProperty("NULL", "OK")
	assert.Equal(t, ok, false)
	assert.Equal(t, v, "OK")

	v = p.GetProperty("INT")
	assert.Equal(t, v, 3)

	var v2 int
	p.BindProperty("int", &v2)
	assert.Equal(t, v2, 3)

	var u2 uint
	p.BindProperty("uint", &u2)
	assert.Equal(t, u2, uint(3))

	var f2 float32
	p.BindProperty("Float", &f2)
	assert.Equal(t, f2, float32(3))

	b := p.GetBoolProperty("BOOL")
	assert.Equal(t, b, true)

	var b2 bool
	p.BindProperty("bool", &b2)
	assert.Equal(t, b2, true)

	i := p.GetIntProperty("INT")
	assert.Equal(t, i, int64(3))

	u := p.GetUintProperty("UINT")
	assert.Equal(t, u, uint64(3))

	f := p.GetFloatProperty("FLOAT")
	assert.Equal(t, f, 3.0)

	s := p.GetStringProperty("STRING")
	assert.Equal(t, s, "3")

	d := p.GetDurationProperty("DURATION")
	assert.Equal(t, d, time.Second*3)

	ti := p.GetTimeProperty("Time")
	assert.Equal(t, ti, time.Date(2020, 02, 04, 20, 02, 04, 0, time.UTC))

	var ss2 []string
	p.BindProperty("[]string", &ss2)
	assert.Equal(t, ss2, []string{"3"})
}

type DB struct {
	UserName string `value:"${username}"`
	Password string `value:"${password}"`
	Url      string `value:"${url}"`
	Port     string `value:"${port}"`
	DB       string `value:"${db}"`
}

type DbConfig struct {
	DB []DB `value:"${db}"`
}

type DBConnection struct {
	UserName string `value:"${username}"`
	Password string `value:"${password}"`
	Url      string `value:"${url}"`
	Port     string `value:"${port}"`
}

type UntaggedNestedDB struct {
	DBConnection `value:"${}"`
	DB           string `value:"${db}"`
}

type TaggedNestedDB struct {
	DBConnection `value:"${tag}"`
	DB           string `value:"${db}"`
}

type TagNestedDbConfig struct {
	DB0 []TaggedNestedDB   `value:"${tagged.db}"`
	DB1 []UntaggedNestedDB `value:"${db}"`
}

type NestedDB struct {
	DBConnection        // 正确，不能有 tag
	DB           string `value:"${db}"`
}

type NestedDbConfig struct {
	DB []NestedDB `value:"${db}"`
}

func TestDefaultProperties_BindProperty(t *testing.T) {

	t.Run("simple bind", func(t *testing.T) {
		p := SpringCore.NewDefaultProperties()
		p.LoadProperties("testdata/config/application.yaml")

		dbConfig1 := DbConfig{}
		p.BindProperty("", &dbConfig1)

		dbConfig2 := DbConfig{}
		p.BindProperty("prefix", &dbConfig2)

		// 实际上是取的两个节点，只是值是一样的而已
		assert.Equal(t, dbConfig1, dbConfig2)
	})

	t.Run("struct bind with tag", func(t *testing.T) {

		p := SpringCore.NewDefaultProperties()
		p.LoadProperties("testdata/config/application.yaml")

		dbConfig := TagNestedDbConfig{}
		p.BindProperty("", &dbConfig)

		fmt.Println(dbConfig)
	})

	t.Run("struct bind without tag", func(t *testing.T) {

		p := SpringCore.NewDefaultProperties()
		p.LoadProperties("testdata/config/application.yaml")

		dbConfig1 := NestedDbConfig{}
		p.BindProperty("", &dbConfig1)

		dbConfig2 := NestedDbConfig{}
		p.BindProperty("prefix", &dbConfig2)

		// 实际上是取的两个节点，只是值是一样的而已
		assert.Equal(t, dbConfig1, dbConfig2)
		assert.Equal(t, len(dbConfig1.DB), 2)
	})
}

type NestedDbMapConfig struct {
	DB map[string]NestedDB `value:"${db_map}"`
}

func TestDefaultProperties_StringMapString(t *testing.T) {

	t.Run("simple bind", func(t *testing.T) {

		p := SpringCore.NewDefaultProperties()
		p.SetProperty("a.b1", "b1")
		p.SetProperty("a.b2", "b2")
		p.SetProperty("a.b3", "b3")

		var m map[string]string
		p.BindProperty("a", &m)

		assert.Equal(t, len(m), 3)
		assert.Equal(t, m["b1"], "b1")
	})

	t.Run("converter bind", func(t *testing.T) {
		SpringCore.RegisterTypeConverter(PointConverter)

		p := SpringCore.NewDefaultProperties()
		p.SetProperty("a.p1", "(1,2)")
		p.SetProperty("a.p2", "(3,4)")
		p.SetProperty("a.p3", "(5,6)")

		var m map[string]image.Point
		p.BindProperty("a", &m)

		assert.Equal(t, len(m), 3)
		assert.Equal(t, m["p1"], image.Pt(1, 2))
	})

	t.Run("simple bind from file", func(t *testing.T) {

		p := SpringCore.NewDefaultProperties()
		p.LoadProperties("testdata/config/application.yaml")

		var m map[string]string
		p.BindProperty("camera", &m)

		assert.Equal(t, len(m), 3)
		assert.Equal(t, m["floor1"], "camera_floor1")
	})

	t.Run("struct bind from file", func(t *testing.T) {

		p := SpringCore.NewDefaultProperties()
		p.LoadProperties("testdata/config/application.yaml")

		var m map[string]NestedDB
		p.BindProperty("db_map", &m)

		assert.Equal(t, len(m), 2)
		assert.Equal(t, m["d1"].DB, "db1")

		dbConfig2 := NestedDbMapConfig{}
		p.BindProperty("prefix_map", &dbConfig2)

		assert.Equal(t, len(dbConfig2.DB), 2)
		assert.Equal(t, dbConfig2.DB["d1"].DB, "db1")
	})
}
