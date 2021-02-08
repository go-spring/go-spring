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

package properties_test

import (
	"errors"
	"fmt"
	"image"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-spring/spring-core/properties"
	"github.com/go-spring/spring-utils"
	"github.com/spf13/cast"
)

func TestDefaultProperties_LoadProperties(t *testing.T) {

	p := properties.New()
	p.Load("testdata/config/application.yaml")
	p.Load("testdata/config/application.properties")

	fmt.Println("Get All Properties:")
	p.Range(func(k string, v interface{}) { fmt.Println(k, v) })
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
			p := properties.New()
			r := strings.NewReader(d.str)
			p.Read(r, "properties")
			v := p.Get(d.key)
			SpringUtils.AssertEqual(t, v, d.val)
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
			p := properties.New()
			r := strings.NewReader(d.str)
			p.Read(r, "properties")
			v := p.Get(d.key)
			SpringUtils.AssertEqual(t, v, d.val)
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

		p := properties.New()
		r := strings.NewReader(str)
		p.Read(r, "properties")

		for k, expect := range data {
			v := p.Get(k)
			SpringUtils.AssertEqual(t, v, expect)
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

		p := properties.New()
		r := strings.NewReader(str)
		p.Read(r, "properties")

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
			SpringUtils.AssertEqual(t, v, expect)
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

		p := properties.New()
		r := strings.NewReader(str)
		p.Read(r, "properties")

		for k, expect := range data {
			v := p.Get(k)
			SpringUtils.AssertEqual(t, v, expect)
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
			{"int", "int: 3", 3 /*int*/, reflect.Int}, // yaml 是 int，toml 是 int64。
			{"float", "float: 3.0", 3.0 /*float64*/, reflect.Float64},
			{"string", "string: \"3\"", "3", reflect.String},
			{"string", "string: hello", "hello", reflect.String},
			{"date", "date: 2018-02-17", "2018-02-17", reflect.String},
			{"time", "time: 2018-02-17T15:02:31+08:00", "2018-02-17T15:02:31+08:00", reflect.String},
		}

		for _, d := range data {
			p := properties.New()
			r := strings.NewReader(d.str)
			p.Read(r, "yaml")
			v := p.Get(d.key)
			SpringUtils.AssertEqual(t, v, d.val)
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
			p := properties.New()
			r := strings.NewReader(d.str)
			p.Read(r, "yaml")
			v := p.Get(d.key)
			SpringUtils.AssertEqual(t, v, d.val)
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

		p := properties.New()
		r := strings.NewReader(str)
		p.Read(r, "yaml")

		for k, expect := range data {
			v := p.Get(k)
			SpringUtils.AssertEqual(t, v, expect)
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

		p := properties.New()
		r := strings.NewReader(str)
		p.Read(r, "yaml")

		v := p.Get("array")
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
		SpringUtils.AssertEqual(t, v, expect)
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

		p := properties.New()
		r := strings.NewReader(str)
		p.Read(r, "yaml")

		for k, expect := range data {
			v := p.Get(k)
			SpringUtils.AssertEqual(t, v, expect)
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
			p := properties.New()
			r := strings.NewReader(d.str)
			p.Read(r, "toml")
			v := p.Get(d.key)
			SpringUtils.AssertEqual(t, v, d.val)
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
			p := properties.New()
			r := strings.NewReader(d.str)
			p.Read(r, "toml")
			v := p.Get(d.key)
			SpringUtils.AssertEqual(t, v, d.val)
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

		p := properties.New()
		r := strings.NewReader(str)
		p.Read(r, "toml")

		for k, expect := range data {
			v := p.Get(k)
			SpringUtils.AssertEqual(t, v, expect)
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

		p := properties.New()
		r := strings.NewReader(str)
		p.Read(r, "toml")

		v := p.Get("array")
		expect := []interface{}{
			map[string]interface{}{ // yaml 是 map[interface{}]interface{}，toml 是 map[string]interface{}
				"bool":   false,
				"int":    int64(3),
				"float":  3.0, /*float64*/
				"string": "hello",
			},
			map[string]interface{}{
				"bool":   true,
				"int":    int64(20),
				"float":  0.2, /*float64*/
				"string": "hello",
			},
		}
		SpringUtils.AssertEqual(t, v, expect)
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
			"map.k1.float":  3.0, /*float64*/
			"map.k1.int":    int64(3),
			"map.k1.string": "hello",
			"map.k2.bool":   true,
			"map.k2.float":  0.2, /*float64*/
			"map.k2.int":    int64(20),
			"map.k2.string": "hello",
		}

		p := properties.New()
		r := strings.NewReader(str)
		p.Read(r, "toml")

		for k, expect := range data {
			v := p.Get(k)
			SpringUtils.AssertEqual(t, v, expect)
		}
	})
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

func TestRegisterTypeConverter(t *testing.T) {
	p := properties.New()

	err := p.Convert(3)
	SpringUtils.AssertEqual(t, err, errors.New("fn must be func(string)(type,error)"))

	err = p.Convert(func(_ string, _ string) (image.Point, error) { return image.Point{}, nil })
	SpringUtils.AssertEqual(t, err, errors.New("fn must be func(string)(type,error)"))

	err = p.Convert(func(_ string) (image.Point, image.Point, error) { return image.Point{}, image.Point{}, nil })
	SpringUtils.AssertEqual(t, err, errors.New("fn must be func(string)(type,error)"))

	err = p.Convert(PointConverter)
	SpringUtils.AssertEqual(t, err, nil)
}

func TestDefaultProperties_GetProperty(t *testing.T) {
	p := properties.New()

	p.Set("a.b.c", "3")
	p.Set("a.b.d", []string{"3"})

	m := p.Prefix("a.b")
	SpringUtils.AssertEqual(t, len(m), 2)
	SpringUtils.AssertEqual(t, m["a.b.c"], "3")
	SpringUtils.AssertEqual(t, m["a.b.d"], []string{"3"})

	p.Set("Bool", true)
	p.Set("Int", 3)
	p.Set("Uint", 3)
	p.Set("Float", 3.0)
	p.Set("String", "3")
	p.Set("Duration", "3s")
	p.Set("[]String", []string{"3"})
	p.Set("Time", "2020-02-04 20:02:04")
	p.Set("[]Map[String]Interface{}", []interface{}{
		map[interface{}]interface{}{
			"1": 2,
		},
	})

	v := p.Get("NULL")
	SpringUtils.AssertEqual(t, v, nil)

	v = p.GetDefault("NULL", "OK")
	SpringUtils.AssertEqual(t, v, "OK")

	v = p.Get("INT")
	SpringUtils.AssertEqual(t, v, 3)

	var v2 int
	p.Bind("int", &v2)
	SpringUtils.AssertEqual(t, v2, 3)

	var u2 uint
	p.Bind("uint", &u2)
	SpringUtils.AssertEqual(t, u2, uint(3))

	var f2 float32
	p.Bind("Float", &f2)
	SpringUtils.AssertEqual(t, f2, float32(3))

	b := cast.ToBool(p.Get("BOOL"))
	SpringUtils.AssertEqual(t, b, true)

	var b2 bool
	p.Bind("bool", &b2)
	SpringUtils.AssertEqual(t, b2, true)

	i := cast.ToInt64(p.Get("INT"))
	SpringUtils.AssertEqual(t, i, int64(3))

	u := cast.ToUint64(p.Get("UINT"))
	SpringUtils.AssertEqual(t, u, uint64(3))

	f := cast.ToFloat64(p.Get("FLOAT"))
	SpringUtils.AssertEqual(t, f, 3.0)

	s := cast.ToString(p.Get("STRING"))
	SpringUtils.AssertEqual(t, s, "3")

	d := cast.ToDuration(p.Get("DURATION"))
	SpringUtils.AssertEqual(t, d, time.Second*3)

	ti := cast.ToTime(p.Get("Time"))
	SpringUtils.AssertEqual(t, ti, time.Date(2020, 02, 04, 20, 02, 04, 0, time.UTC))

	var ss2 []string
	p.Bind("[]string", &ss2)
	SpringUtils.AssertEqual(t, ss2, []string{"3"})
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
		p := properties.New()
		p.Load("testdata/config/application.yaml")

		dbConfig1 := DbConfig{}
		p.Bind("", &dbConfig1)

		dbConfig2 := DbConfig{}
		p.Bind("prefix", &dbConfig2)

		// 实际上是取的两个节点，只是值是一样的而已
		SpringUtils.AssertEqual(t, dbConfig1, dbConfig2)
	})

	t.Run("struct bind with tag", func(t *testing.T) {

		p := properties.New()
		p.Load("testdata/config/application.yaml")

		dbConfig := TagNestedDbConfig{}
		p.Bind("", &dbConfig)

		fmt.Println(dbConfig)
	})

	t.Run("struct bind without tag", func(t *testing.T) {

		p := properties.New()
		p.Load("testdata/config/application.yaml")

		dbConfig1 := NestedDbConfig{}
		p.Bind("", &dbConfig1)

		dbConfig2 := NestedDbConfig{}
		p.Bind("prefix", &dbConfig2)

		// 实际上是取的两个节点，只是值是一样的而已
		SpringUtils.AssertEqual(t, dbConfig1, dbConfig2)
		SpringUtils.AssertEqual(t, len(dbConfig1.DB), 2)
	})
}

type NestedDbMapConfig struct {
	DB map[string]NestedDB `value:"${db_map}"`
}

func TestDefaultProperties_StringMapString(t *testing.T) {

	t.Run("simple map bind", func(t *testing.T) {

		p := properties.New()
		p.Set("a.b1", "b1")
		p.Set("a.b2", "b2")
		p.Set("a.b3", "b3")

		var m map[string]string
		p.Bind("a", &m)

		SpringUtils.AssertEqual(t, len(m), 3)
		SpringUtils.AssertEqual(t, m["b1"], "b1")
	})

	t.Run("converter bind", func(t *testing.T) {

		p := properties.New()
		p.Convert(PointConverter)
		p.Set("a.p1", "(1,2)")
		p.Set("a.p2", "(3,4)")
		p.Set("a.p3", "(5,6)")

		var m map[string]image.Point
		p.Bind("a", &m)

		SpringUtils.AssertEqual(t, len(m), 3)
		SpringUtils.AssertEqual(t, m["p1"], image.Pt(1, 2))
	})

	t.Run("simple bind from file", func(t *testing.T) {

		p := properties.New()
		p.Load("testdata/config/application.yaml")

		var m map[string]string
		p.Bind("camera", &m)

		SpringUtils.AssertEqual(t, len(m), 3)
		SpringUtils.AssertEqual(t, m["floor1"], "camera_floor1")
	})

	t.Run("struct bind from file", func(t *testing.T) {

		p := properties.New()
		p.Load("testdata/config/application.yaml")

		var m map[string]NestedDB
		p.Bind("db_map", &m)

		SpringUtils.AssertEqual(t, len(m), 2)
		SpringUtils.AssertEqual(t, m["d1"].DB, "db1")

		dbConfig2 := NestedDbMapConfig{}
		p.Bind("prefix_map", &dbConfig2)

		SpringUtils.AssertEqual(t, len(dbConfig2.DB), 2)
		SpringUtils.AssertEqual(t, dbConfig2.DB["d1"].DB, "db1")
	})
}

func TestDefaultProperties_ConfigRef(t *testing.T) {

	type fileLog struct {
		Dir             string `value:"${dir:=${app.dir}}"`
		NestedDir       string `value:"${nested.dir:=${nested.app.dir:=./log}}"`
		NestedEmptyDir  string `value:"${nested.dir:=${nested.app.dir:=}}"`
		NestedNestedDir string `value:"${nested.dir:=${nested.app.dir:=${nested.nested.app.dir:=./log}}}"`
	}

	var mqLog struct{ fileLog }
	var httpLog struct{ fileLog }

	t.Run("not config", func(t *testing.T) {
		p := properties.New()
		err := p.Bind("", &httpLog)
		SpringUtils.AssertEqual(t, err, errors.New("property \"app.dir\" not config"))
	})

	t.Run("config", func(t *testing.T) {
		p := properties.New()

		appDir := "/home/log"
		p.Set("app.dir", appDir)

		p.Bind("", &httpLog)
		SpringUtils.AssertEqual(t, httpLog.Dir, appDir)
		SpringUtils.AssertEqual(t, httpLog.NestedDir, "./log")
		SpringUtils.AssertEqual(t, httpLog.NestedEmptyDir, "")
		SpringUtils.AssertEqual(t, httpLog.NestedNestedDir, "./log")

		p.Bind("", &mqLog)
		SpringUtils.AssertEqual(t, mqLog.Dir, appDir)
		SpringUtils.AssertEqual(t, mqLog.NestedDir, "./log")
		SpringUtils.AssertEqual(t, mqLog.NestedEmptyDir, "")
		SpringUtils.AssertEqual(t, mqLog.NestedNestedDir, "./log")
	})
}

func TestDefaultProperties_KeyCanBeEmpty(t *testing.T) {
	p := properties.New()
	var s struct {
		KeyIsEmpty string `value:"${:=kie}"`
	}
	p.Bind("", &s)
	SpringUtils.AssertEqual(t, s.KeyIsEmpty, "kie")
}
