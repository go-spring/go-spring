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

package bind_2_test

import (
	"container/list"
	"fmt"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/conf"
	"github.com/go-spring/spring-base/log"
)

func init() {
	log.SetLevel(log.TraceLevel)
}

type DB struct {
	username string // 相当于 `value:"${username}"`
	password string // 相当于 `value:"${password}"`
	url      string // 相当于 `value:"${url}"`
	port     string // 相当于 `value:"${port}"`
	db       string // 相当于 `value:"${db}"`
}

type DbConfig struct {
	db []DB // 相当于 `value:"${db}"`
}

type DBConnection struct {
	username string // 相当于 `value:"${username}"`
	password string // 相当于 `value:"${password}"`
	url      string // 相当于 `value:"${url}"`
	port     string // 相当于 `value:"${port}"`
}

type UntaggedNestedDB struct {
	DBConnection
	db string // 相当于 `value:"${db}"`
}

type TaggedNestedDB struct {
	DBConnection `value:"${tag}"`
	db           string // 相当于 `value:"${db}"`
}

type TagNestedDbConfig struct {
	DB0 []TaggedNestedDB   `value:"${tagged.db}"`
	db  []UntaggedNestedDB // 相当于 `value:"${db}"`
}

type NestedDB struct {
	DBConnection
	db string // 相当于 `value:"${db}"`
}

type NestedDbConfig struct {
	db []NestedDB // 相当于 `value:"${db}"`
}

type NestedDbMapConfig struct {
	DBMap map[string]NestedDB // 相当于 `value:"${DBMap}"`
}

func TestProperties_Bind(t *testing.T) {

	t.Run("default", func(t *testing.T) {
		p := conf.New()
		v := &struct {
			S struct {
				V int `value:"${:=3}"`
			} `value:"${s:=}"`
		}{}
		err := p.Bind(v)
		assert.Nil(t, err)
		assert.Equal(t, v.S.V, 3)
	})

	t.Run("simple bind", func(t *testing.T) {
		p, err := conf.Load("testdata/config/application.yaml")
		assert.Nil(t, err)

		dbConfig1 := DbConfig{}
		err = p.Bind(&dbConfig1)
		assert.Nil(t, err)

		dbConfig2 := DbConfig{}
		err = p.Bind(&dbConfig2, conf.Tag("${prefix}"))
		assert.Nil(t, err)

		// 实际上是取的两个节点，只是值是一样的而已
		assert.Equal(t, dbConfig1, dbConfig2)
	})

	t.Run("struct bind with tag", func(t *testing.T) {

		p, err := conf.Load("testdata/config/application.yaml")
		assert.Nil(t, err)

		dbConfig := TagNestedDbConfig{}
		err = p.Bind(&dbConfig)
		assert.Nil(t, err)

		fmt.Println(dbConfig)
	})

	t.Run("struct bind without tag", func(t *testing.T) {

		p, err := conf.Load("testdata/config/application.yaml")
		assert.Nil(t, err)

		dbConfig1 := NestedDbConfig{}
		err = p.Bind(&dbConfig1)
		assert.Nil(t, err)

		dbConfig2 := NestedDbConfig{}
		err = p.Bind(&dbConfig2, conf.Tag("${prefix}"))
		assert.Nil(t, err)

		// 实际上是取的两个节点，只是值是一样的而已
		assert.Equal(t, dbConfig1, dbConfig2)
		assert.Equal(t, len(dbConfig1.db), 2)
	})

	t.Run("simple map bind", func(t *testing.T) {

		p := conf.New()
		err := p.Set("a.b1", "b1")
		assert.Nil(t, err)
		err = p.Set("a.b2", "b2")
		assert.Nil(t, err)
		err = p.Set("a.b3", "b3")
		assert.Nil(t, err)

		var m map[string]string
		err = p.Bind(&m, conf.Tag("${a}"))
		assert.Nil(t, err)

		assert.Equal(t, len(m), 3)
		assert.Equal(t, m["b1"], "b1")
	})

	t.Run("simple bind from file", func(t *testing.T) {

		p, err := conf.Load("testdata/config/application.yaml")
		assert.Nil(t, err)

		var m map[string]string
		err = p.Bind(&m, conf.Tag("${camera}"))
		assert.Nil(t, err)

		assert.Equal(t, len(m), 3)
		assert.Equal(t, m["floor1"], "camera_floor1")
	})

	t.Run("struct bind from file", func(t *testing.T) {

		p, err := conf.Load("testdata/config/application.yaml")
		assert.Nil(t, err)

		var m map[string]NestedDB
		err = p.Bind(&m, conf.Tag("${DBMap}"))
		assert.Nil(t, err)

		assert.Equal(t, len(m), 2)
		assert.Equal(t, m["d1"].db, "db1")

		dbConfig2 := NestedDbMapConfig{}
		err = p.Bind(&dbConfig2, conf.Tag("${prefix_map}"))
		assert.Nil(t, err)

		assert.Equal(t, len(dbConfig2.DBMap), 2)
		assert.Equal(t, dbConfig2.DBMap["d1"].db, "db1")
	})

	t.Run("ignore interface", func(t *testing.T) {
		p := conf.New()
		err := p.Bind(&struct{ fmt.Stringer }{})
		assert.Nil(t, err)
	})

	t.Run("ignore pointer", func(t *testing.T) {
		p := conf.New()
		err := p.Bind(list.New())
		assert.Error(t, err, "bind.go:87 type \"int\" bind error\nbind.go:437 property \"len\" not exist")
	})
}
