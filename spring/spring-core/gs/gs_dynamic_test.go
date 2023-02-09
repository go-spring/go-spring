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

package gs_test

import (
	"encoding/json"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/dync"
	"github.com/go-spring/spring-core/gs"
)

type DynamicConfig struct {
	Int   dync.Int64   `value:"${int:=3}" expr:"$<6"`
	Float dync.Float64 `value:"${float:=1.2}"`
	Map   dync.Ref     `value:"${map:=}"`
	Slice dync.Ref     `value:"${slice:=}"`
}

type DynamicConfigWrapper struct {
	Wrapper DynamicConfig `value:"${wrapper}"`
}

func TestDynamic(t *testing.T) {

	c := gs.New()

	wrapper := new(DynamicConfigWrapper)
	wrapper.Wrapper.Slice.Init(make([]string, 0))
	wrapper.Wrapper.Map.Init(make(map[string]string))
	c.Object(wrapper)

	var cfg *DynamicConfig
	c.Provide(func() *DynamicConfig {
		config := new(DynamicConfig)
		config.Slice.Init(make([]string, 0))
		config.Map.Init(make(map[string]string))
		return config
	}).Init(func(config *DynamicConfig) {
		cfg = config
	})

	err := c.Refresh()
	assert.Nil(t, err)

	{
		b, _ := json.Marshal(cfg)
		assert.Equal(t, string(b), `{"Int":3,"Float":1.2,"Map":{},"Slice":[]}`)
		b, _ = json.Marshal(wrapper)
		assert.Equal(t, string(b), `{"Wrapper":{"Int":3,"Float":1.2,"Map":{},"Slice":[]}}`)
	}

	{
		p := conf.New()
		p.Set("int", 4)
		p.Set("float", 2.3)
		p.Set("map.a", 1)
		p.Set("map.b", 2)
		p.Set("slice[0]", 3)
		p.Set("slice[1]", 4)
		p.Set("wrapper.int", 3)
		p.Set("wrapper.float", 1.5)
		p.Set("wrapper.map.a", 9)
		p.Set("wrapper.map.b", 8)
		p.Set("wrapper.slice[0]", 4)
		p.Set("wrapper.slice[1]", 6)
		c.Properties().Refresh(p)
	}

	{
		b, _ := json.Marshal(cfg)
		assert.Equal(t, string(b), `{"Int":4,"Float":2.3,"Map":{"a":"1","b":"2"},"Slice":["3","4"]}`)
		b, _ = json.Marshal(wrapper)
		assert.Equal(t, string(b), `{"Wrapper":{"Int":3,"Float":1.5,"Map":{"a":"9","b":"8"},"Slice":["4","6"]}}`)
	}

	{
		p := conf.New()
		p.Set("int", 6)
		p.Set("float", 5.1)
		p.Set("map.a", 9)
		p.Set("map.b", 8)
		p.Set("slice[0]", 7)
		p.Set("slice[1]", 6)
		p.Set("wrapper.int", 9)
		p.Set("wrapper.float", 8.4)
		p.Set("wrapper.map.a", 3)
		p.Set("wrapper.map.b", 4)
		p.Set("wrapper.slice[0]", 2)
		p.Set("wrapper.slice[1]", 1)
		err = c.Properties().Refresh(p)
		assert.Error(t, err, "validate failed on \"\\$<6\" for value 6")
	}

	{
		b, _ := json.Marshal(cfg)
		assert.Equal(t, string(b), `{"Int":4,"Float":2.3,"Map":{"a":"1","b":"2"},"Slice":["3","4"]}`)
		b, _ = json.Marshal(wrapper)
		assert.Equal(t, string(b), `{"Wrapper":{"Int":3,"Float":1.5,"Map":{"a":"9","b":"8"},"Slice":["4","6"]}}`)
	}

	{
		p := conf.New()
		p.Set("int", 1)
		p.Set("float", 5.1)
		p.Set("map.a", 9)
		p.Set("map.b", 8)
		p.Set("slice[0]", 7)
		p.Set("slice[1]", 6)
		p.Set("wrapper.int", 9)
		p.Set("wrapper.float", 8.4)
		p.Set("wrapper.map.a", 3)
		p.Set("wrapper.map.b", 4)
		p.Set("wrapper.slice[0]", 2)
		p.Set("wrapper.slice[1]", 1)
		err = c.Properties().Refresh(p)
		assert.Error(t, err, "validate failed on \\\"\\$<6\\\" for value 9")
	}

	{
		b, _ := json.Marshal(cfg)
		assert.Equal(t, string(b), `{"Int":1,"Float":5.1,"Map":{"a":"9","b":"8"},"Slice":["7","6"]}`)
		b, _ = json.Marshal(wrapper)
		assert.Equal(t, string(b), `{"Wrapper":{"Int":3,"Float":1.5,"Map":{"a":"9","b":"8"},"Slice":["4","6"]}}`)
	}
}
