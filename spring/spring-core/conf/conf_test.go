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
	"container/list"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/util"
	"github.com/spf13/cast"
)

func init() {
	log.SetLevel(log.TraceLevel)
}

//func TestProperties_Load(t *testing.T) {
//
//	p := conf.New()
//	err := p.Load("testdata/config/application.yaml")
//	util.Panic(err).When(err != nil)
//	err = p.Load("testdata/config/application.properties")
//	util.Panic(err).When(err != nil)
//
//	m := p.FlatMap()
//
//	var keys []string
//	for key := range m {
//		keys = append(keys, key)
//	}
//
//	sort.Strings(keys)
//
//	for _, k := range keys {
//		fmt.Println(k+":", m[k])
//	}
//}

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
			p, _ := conf.Read([]byte(d.str), ".properties")
			v := p.Get(d.key)
			assert.Equal(t, v, d.val)
		}
	})

	// 目前使用的 properties 解析库不支持数组
	//t.Run("array", func(t *testing.T) {
	//
	//	data := []struct {
	//		key  string
	//		str  string
	//		val  interface{}
	//		kind reflect.Kind
	//	}{
	//		{"bool[0]", "bool[0]=false", "false", reflect.String},
	//		{"int[0]", "int[0]=3", "3", reflect.String},
	//		{"float[0]", "float[0]=3.0", "3.0", reflect.String},
	//		{"string[0]", "string[0]=\"3\"", "\"3\"", reflect.String},
	//		{"string[0]", "string[0]=hello", "hello", reflect.String},
	//	}
	//
	//	for _, d := range data {
	//		p, _ := conf.Read([]byte(d.str), ".properties")
	//		v,_ := p.Get(d.key)
	//		assert.Equal(t, v, d.val)
	//	}
	//})

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

		p, _ := conf.Read([]byte(str), ".properties")
		for k, expect := range data {
			v := p.Get(k)
			assert.Equal(t, v, expect)
		}
	})

	// 目前使用的 properties 解析库不支持数组
	//t.Run("array struct", func(t *testing.T) {
	//
	//	str := `
	//          array[0].bool=false
	//          array[0].int=3
	//          array[0].float=3.0
	//          array[0].string=hello
	//          array[1].bool=true
	//          array[1].int=20
	//          array[1].float=0.2
	//          array[1].string=hello
	//        `
	//
	//	p, _ := conf.Read([]byte(str), ".properties")
	//	data := map[string]interface{}{
	//		"array[0].bool":   "false",
	//		"array[0].int":    "3",
	//		"array[0].float":  "3.0",
	//		"array[0].string": "hello",
	//		"array[1].bool":   "true",
	//		"array[1].int":    "20",
	//		"array[1].float":  "0.2",
	//		"array[1].string": "hello",
	//	}
	//
	//	for k, expect := range data {
	//		v,_ := p.Get(k)
	//		assert.Equal(t, v, expect)
	//	}
	//})

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

		p, _ := conf.Read([]byte(str), ".properties")
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
			p, _ := conf.Read([]byte(d.str), ".yaml")
			v := p.Get(d.key)
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
			{"bool[0]", "bool: [false]", "false", reflect.Bool},
			{"int[0]", "int: [3]", "3", reflect.Int},
			{"float[0]", "float: [3.0]", "3", reflect.Float64},
			{"string[0]", "string: [\"3\"]", "3", reflect.String},
			{"string[0]", "string: [hello]", "hello", reflect.String},
		}

		for _, d := range data {
			p, _ := conf.Read([]byte(d.str), ".yaml")
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

		p, _ := conf.Read([]byte(str), ".yaml")
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

		p, _ := conf.Read([]byte(str), ".yaml")

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

		p, _ := conf.Read([]byte(str), ".yaml")

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
			p, _ := conf.Read([]byte(d.str), ".toml")
			v := p.Get(d.key)
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
			{"bool[0]", "bool=[false]", "false", reflect.Bool},
			{"int[0]", "int=[3]", "3", reflect.Int},
			{"float[0]", "float=[3.0]", "3", reflect.Float64},
			{"string[0]", "string=[\"3\"]", "3", reflect.String},
			{"string[0]", "string=[\"hello\"]", "hello", reflect.String},
		}

		for _, d := range data {
			p, _ := conf.Read([]byte(d.str), ".toml")
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

		p, _ := conf.Read([]byte(str), ".toml")
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

		p, _ := conf.Read([]byte(str), ".toml")

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

		p, _ := conf.Read([]byte(str), ".toml")

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

		p.Set("a.b.c", "3")
		p.Set("a.b.d", []string{"3"})

		v := p.Get("a.b.c")
		assert.Equal(t, v, "3")
		v = p.Get("a.b.d[0]")
		assert.Equal(t, v, "3")

		p.Set("Bool", true)
		p.Set("Int", 3)
		p.Set("Uint", 3)
		p.Set("Float", 3.0)
		p.Set("String", "3")
		p.Set("Duration", "3s")
		p.Set("StringSlice", []string{"3", "4"})
		p.Set("Time", "2020-02-04 20:02:04")
		p.Set("MapStringInterface", []interface{}{
			map[interface{}]interface{}{
				"1": 2,
			},
		})

		v = p.Get("NULL")
		assert.Equal(t, v, "")

		v = p.Get("NULL", conf.Def("OK"))
		assert.Equal(t, v, "OK")

		v = p.Get("INT")
		assert.Equal(t, v, "3")

		var v2 int
		err := conf.Bind(p, &v2, conf.Key("int"))
		util.Panic(err).When(err != nil)
		assert.Equal(t, v2, 3)

		var u2 uint
		err = conf.Bind(p, &u2, conf.Key("uint"))
		util.Panic(err).When(err != nil)
		assert.Equal(t, u2, uint(3))

		var u3 uint32
		err = conf.Bind(p, &u3, conf.Key("uint"))
		util.Panic(err).When(err != nil)
		assert.Equal(t, u3, uint32(3))

		var f2 float32
		err = conf.Bind(p, &f2, conf.Key("Float"))
		util.Panic(err).When(err != nil)
		assert.Equal(t, f2, float32(3))

		v = p.Get("BOOL")
		b := cast.ToBool(v)
		assert.Equal(t, b, true)

		var b2 bool
		err = conf.Bind(p, &b2, conf.Key("bool"))
		util.Panic(err).When(err != nil)
		assert.Equal(t, b2, true)

		v = p.Get("INT")
		i := cast.ToInt64(v)
		assert.Equal(t, i, int64(3))

		v = p.Get("UINT")
		u := cast.ToUint64(v)
		assert.Equal(t, u, uint64(3))

		v = p.Get("FLOAT")
		f := cast.ToFloat64(v)
		assert.Equal(t, f, 3.0)

		v = p.Get("STRING")
		s := cast.ToString(v)
		assert.Equal(t, s, "3")

		v = p.Get("DURATION")
		d := cast.ToDuration(v)
		assert.Equal(t, d, time.Second*3)

		v = p.Get("Time")
		ti := cast.ToTime(v)
		assert.Equal(t, ti, time.Date(2020, 02, 04, 20, 02, 04, 0, time.UTC))

		var ss2 []string
		err = conf.Bind(p, &ss2, conf.Key("StringSlice"))
		util.Panic(err).When(err != nil)
		assert.Equal(t, ss2, []string{"3", "4"})
	})

	t.Run("slice slice", func(t *testing.T) {
		p := conf.Map(map[string]interface{}{
			"a": []interface{}{
				[]interface{}{1, 2},
				[]interface{}{3, 4},
				map[string]interface{}{
					"b": "c",
					"d": []interface{}{5, 6},
				},
			},
		})
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
		v = p.Get("a[2].d[0]")
		assert.Equal(t, v, "5")
		v = p.Get("a[2].d[1]")
		assert.Equal(t, v, "6")
		v = p.Get("a[2].d[1]")
		assert.Equal(t, v, "6")
		ok := p.Has("a[2].d[2]")
		assert.Equal(t, ok, false)
	})
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
	DBConnection
	DB string `value:"${db}"`
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
	DBConnection
	DB string `value:"${db}"`
}

type NestedDbConfig struct {
	DB      []NestedDB     `value:"${db}"`
	Ints    []int          `value:"${:=}"`
	Map     map[string]int `value:"${:=}"`
	Structs []struct {
		V string `value:"${v:=#v}"`
	} `value:"${:=}"`
}

type NestedDbMapConfig struct {
	DB map[string]NestedDB `value:"${db_map}"`
}

func TestProperties_Bind(t *testing.T) {

	t.Run("default", func(t *testing.T) {
		p := conf.New()
		v := &struct {
			S struct {
				V int `value:"${:=3}"`
			} `value:"${s:=}"`
		}{}
		err := conf.Bind(p, v)
		util.Panic(err).When(err != nil)
		assert.Equal(t, v.S.V, 3)
	})

	t.Run("simple bind", func(t *testing.T) {
		p, err := conf.Load("testdata/config/application.yaml")
		util.Panic(err).When(err != nil)

		dbConfig1 := DbConfig{}
		err = conf.Bind(p, &dbConfig1)
		util.Panic(err).When(err != nil)

		dbConfig2 := DbConfig{}
		err = conf.Bind(p, &dbConfig2, conf.Tag("${prefix}"))
		util.Panic(err).When(err != nil)

		// 实际上是取的两个节点，只是值是一样的而已
		assert.Equal(t, dbConfig1, dbConfig2)
	})

	t.Run("struct bind with tag", func(t *testing.T) {

		p, err := conf.Load("testdata/config/application.yaml")
		util.Panic(err).When(err != nil)

		dbConfig := TagNestedDbConfig{}
		err = conf.Bind(p, &dbConfig)
		util.Panic(err).When(err != nil)

		fmt.Println(dbConfig)
	})

	t.Run("struct bind without tag", func(t *testing.T) {

		p, err := conf.Load("testdata/config/application.yaml")
		util.Panic(err).When(err != nil)

		dbConfig1 := NestedDbConfig{}
		err = conf.Bind(p, &dbConfig1)
		util.Panic(err).When(err != nil)

		dbConfig2 := NestedDbConfig{}
		err = conf.Bind(p, &dbConfig2, conf.Tag("${prefix}"))
		util.Panic(err).When(err != nil)

		// 实际上是取的两个节点，只是值是一样的而已
		assert.Equal(t, dbConfig1, dbConfig2)
		assert.Equal(t, len(dbConfig1.DB), 2)
	})

	t.Run("simple map bind", func(t *testing.T) {

		p := conf.New()
		p.Set("a.b1", "b1")
		p.Set("a.b2", "b2")
		p.Set("a.b3", "b3")

		var m map[string]string
		err := conf.Bind(p, &m, conf.Tag("${a}"))
		util.Panic(err).When(err != nil)

		assert.Equal(t, len(m), 3)
		assert.Equal(t, m["b1"], "b1")
	})

	t.Run("simple bind from file", func(t *testing.T) {

		p, err := conf.Load("testdata/config/application.yaml")
		util.Panic(err).When(err != nil)

		var m map[string]string
		err = conf.Bind(p, &m, conf.Tag("${camera}"))
		util.Panic(err).When(err != nil)

		assert.Equal(t, len(m), 3)
		assert.Equal(t, m["floor1"], "camera_floor1")
	})

	t.Run("struct bind from file", func(t *testing.T) {

		p, err := conf.Load("testdata/config/application.yaml")
		util.Panic(err).When(err != nil)

		var m map[string]NestedDB
		err = conf.Bind(p, &m, conf.Tag("${db_map}"))
		util.Panic(err).When(err != nil)

		assert.Equal(t, len(m), 2)
		assert.Equal(t, m["d1"].DB, "db1")

		dbConfig2 := NestedDbMapConfig{}
		err = conf.Bind(p, &dbConfig2, conf.Tag("${prefix_map}"))
		util.Panic(err).When(err != nil)

		assert.Equal(t, len(dbConfig2.DB), 2)
		assert.Equal(t, dbConfig2.DB["d1"].DB, "db1")
	})

	t.Run("ignore interface", func(t *testing.T) {
		p := conf.New()
		err := conf.Bind(p, &struct{ fmt.Stringer }{})
		assert.Nil(t, err)
	})

	t.Run("ignore pointer", func(t *testing.T) {
		p := conf.New()
		err := conf.Bind(p, list.New())
		assert.Nil(t, err)
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
		err := conf.Bind(p, &httpLog)
		assert.Error(t, err, "property \\\"app.dir\\\" not exist")
	})

	t.Run("config", func(t *testing.T) {
		p := conf.New()

		appDir := "/home/log"
		p.Set("app.dir", appDir)

		err := conf.Bind(p, &httpLog)
		util.Panic(err).When(err != nil)
		assert.Equal(t, httpLog.Dir, appDir)
		assert.Equal(t, httpLog.NestedDir, "./log")
		assert.Equal(t, httpLog.NestedEmptyDir, "")
		assert.Equal(t, httpLog.NestedNestedDir, "./log")

		err = conf.Bind(p, &mqLog)
		util.Panic(err).When(err != nil)
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
		err := conf.Bind(p, &s)
		util.Panic(err).When(err != nil)
		assert.Equal(t, s.KeyIsEmpty, "kie")
	})
}

func TestBindMap(t *testing.T) {
	m := map[string]interface{}{
		"a.b1": "b1",
		"a.b2": "b2",
		"a.b3": "b3",
	}
	var r map[string]struct {
		B1 string `value:"${b1}"`
		B2 string `value:"${b2}"`
		B3 string `value:"${b3}"`
	}
	p := conf.Map(m)
	err := conf.Bind(p, &r)
	assert.Nil(t, err)
	assert.Equal(t, r["a"].B1, "b1")
}
