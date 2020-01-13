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
	"strings"
	"testing"

	"github.com/go-spring/go-spring/spring-core"
	"github.com/magiconair/properties/assert"
	"github.com/spf13/cast"
)

func TestMap(t *testing.T) {
	m := make(map[string]interface{})

	// 使用判断模式
	v, ok := m["aaa"]
	fmt.Println(v, ok)

	// 不存在时返回 nil
	fmt.Println(m["aaa"])
}

func TestDefaultProperties_LoadProperties(t *testing.T) {

	p := SpringCore.NewDefaultProperties()
	p.LoadProperties("testdata/config/application.yaml")
	p.LoadProperties("testdata/config/application.properties")

	fmt.Println(">>> GetAllProperties")
	for k, v := range p.GetAllProperties() {
		fmt.Println(k, v)
	}
}

type Point struct {
	x int
	y int
}

type PointBean struct {
	Point        Point `value:"${point}"`
	DefaultPoint Point `value:"${default_point:=(3,4)}"`

	PointList []Point `value:"${point.list}"`
}

func PointConverter(val string) Point {
	if !(strings.HasPrefix(val, "(") && strings.HasSuffix(val, ")")) {
		panic(errors.New("数据格式错误"))
	}
	ss := strings.Split(val[1:len(val)-1], ",")
	x := cast.ToInt(ss[0])
	y := cast.ToInt(ss[1])
	return Point{x, y}
}

func TestRegisterTypeConverter(t *testing.T) {

	assert.Panic(t, func() { // 不是函数
		SpringCore.RegisterTypeConverter(3)
	}, "fn must be func\\(string\\)type")

	assert.Panic(t, func() { // 入参太多
		SpringCore.RegisterTypeConverter(func(_ string, _ string) Point {
			return Point{}
		})
	}, "fn must be func\\(string\\)type")

	assert.Panic(t, func() { // 返回值太多
		SpringCore.RegisterTypeConverter(func(_ string) (Point, Point) {
			return Point{}, Point{}
		})
	}, "fn must be func\\(string\\)type")

	SpringCore.RegisterTypeConverter(PointConverter)
}

func TestDefaultProperties_GetProperty(t *testing.T) {
	p := SpringCore.NewDefaultProperties()

	p.SetProperty("Bool", true)
	p.SetProperty("Int", 3)
	p.SetProperty("Uint", 3)
	p.SetProperty("Float", 3.0)
	p.SetProperty("String", "3")
	p.SetProperty("[]String", []string{"3"})
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

	var ss2 []string
	p.BindProperty("[]string", &ss2)
	assert.Equal(t, ss2, []string{"3"})
}

func TestDefaultProperties_GetPrefixProperties(t *testing.T) {
	p := SpringCore.NewDefaultProperties()
	p.SetProperty("a.b.c", "3")
	p.SetProperty("a.b.d", []string{"3"})
	m := p.GetPrefixProperties("a.b")
	assert.Equal(t, len(m), 2)
	assert.Equal(t, m["a.b.c"], "3")
	assert.Equal(t, m["a.b.d"], []string{"3"})
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

func TestDefaultProperties_BindProperty(t *testing.T) {

	p := SpringCore.NewDefaultProperties()
	p.LoadProperties("testdata/config/application.yaml")

	dbConfig1 := DbConfig{}
	p.BindProperty("", &dbConfig1)

	dbConfig2 := DbConfig{}
	p.BindProperty("prefix", &dbConfig2)

	// 实际上是取的两个节点，只是值是一样的而已
	assert.Equal(t, dbConfig1, dbConfig2)
}

type DBConnection struct {
	UserName string `value:"${username}"`
	Password string `value:"${password}"`
	Url      string `value:"${url}"`
	Port     string `value:"${port}"`
}

type ErrorNestedDB struct {
	DBConnection `value:"${db}"` // 错误，不能有 tag
	DB string    `value:"${db}"`
}

type ErrorNestedDbConfig struct {
	DB []ErrorNestedDB `value:"${db}"`
}

type NestedDB struct {
	DBConnection // 正确，不能有 tag
	DB string `value:"${db}"`
}

type NestedDbConfig struct {
	DB []NestedDB `value:"${db}"`
}

func TestDefaultProperties_GetAllProperties(t *testing.T) {

	t.Run("error", func(t *testing.T) {

		p := SpringCore.NewDefaultProperties()
		p.LoadProperties("testdata/config/application.yaml")

		assert.Panic(t, func() {
			dbConfig1 := ErrorNestedDbConfig{}
			p.BindProperty("", &dbConfig1)
		}, "ErrorNestedDbConfig.\\$DB.\\$DBConnection 嵌套结构体上不允许有 value 标签")
	})

	t.Run("success", func(t *testing.T) {

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

func TestDefaultProperties_GetStringMapStringProperty(t *testing.T) {

	t.Run("set property", func(t *testing.T) {

		p := SpringCore.NewDefaultProperties()
		p.SetProperty("a.b1", "b1")
		p.SetProperty("a.b2", "b2")
		p.SetProperty("a.b3", "b3")

		var m map[string]string
		p.BindProperty("a", &m)

		assert.Equal(t, len(m), 3)
		assert.Equal(t, m["b1"], "b1")
	})

	t.Run("set property converter", func(t *testing.T) {
		SpringCore.RegisterTypeConverter(PointConverter)

		p := SpringCore.NewDefaultProperties()
		p.SetProperty("a.p1", "(1,2)")
		p.SetProperty("a.p2", "(3,4)")
		p.SetProperty("a.p3", "(5,6)")

		var m map[string]Point
		p.BindProperty("a", &m)

		assert.Equal(t, len(m), 3)
		assert.Equal(t, m["p1"], Point{1, 2})
	})

	t.Run("load from file", func(t *testing.T) {

		p := SpringCore.NewDefaultProperties()
		p.LoadProperties("testdata/config/application.yaml")

		var m map[string]string
		p.BindProperty("camera", &m)

		assert.Equal(t, len(m), 3)
		assert.Equal(t, m["floor1"], "camera_floor1")
	})

	t.Run("load from file struct", func(t *testing.T) {

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
