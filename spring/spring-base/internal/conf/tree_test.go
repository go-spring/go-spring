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
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/internal/conf"
)

func TestParse(t *testing.T) {

	t.Run("nil #1", func(t *testing.T) {
		node, err := conf.Parse(nil)
		assert.Nil(t, err)
		assert.Equal(t, node, &conf.NilNode{})
	})

	t.Run("nil #2", func(t *testing.T) {
		node, err := conf.Parse([]interface{}{nil, true})
		assert.Nil(t, err)
		assert.Equal(t, node, &conf.ArrayNode{
			Data: []conf.Node{
				&conf.NilNode{},
				&conf.ValueNode{Data: "true"},
			},
		})
	})

	t.Run("nil #3", func(t *testing.T) {
		node, err := conf.Parse(map[string]interface{}{"a": nil})
		assert.Nil(t, err)
		assert.Equal(t, node, &conf.MapNode{
			Data: map[string]conf.Node{
				"a": &conf.NilNode{},
			},
		})
	})

	t.Run("value #1", func(t *testing.T) {
		node, err := conf.Parse(3)
		assert.Nil(t, err)
		assert.Equal(t, node, &conf.ValueNode{Data: "3"})
	})

	t.Run("slice #1", func(t *testing.T) {
		node, err := conf.Parse([]int{3})
		assert.Nil(t, err)
		assert.Equal(t, node, &conf.ArrayNode{Data: []conf.Node{
			&conf.ValueNode{Data: "3"},
		}})
	})

	t.Run("slice #2", func(t *testing.T) {
		node, err := conf.Parse(map[string]interface{}{"0": 3})
		assert.Nil(t, err)
		assert.Equal(t, node, &conf.ArrayNode{Data: []conf.Node{
			&conf.ValueNode{Data: "3"},
		}})
	})

	t.Run("slice #3", func(t *testing.T) {
		node, err := conf.Parse(map[string]interface{}{
			"0": []interface{}{3},
		})
		assert.Nil(t, err)
		assert.Equal(t, node, &conf.ArrayNode{Data: []conf.Node{
			&conf.ArrayNode{Data: []conf.Node{
				&conf.ValueNode{Data: "3"},
			}},
		}})
	})

	t.Run("slice #4", func(t *testing.T) {
		node, err := conf.Parse(map[string]interface{}{
			"1": []interface{}{3},
		})
		assert.Nil(t, err)
		assert.Equal(t, node, &conf.ArrayNode{Data: []conf.Node{
			nil,
			&conf.ArrayNode{Data: []conf.Node{
				&conf.ValueNode{Data: "3"},
			}},
		}})
	})

	t.Run("map #1", func(t *testing.T) {
		node, err := conf.Parse(map[string]interface{}{"a": 3})
		assert.Nil(t, err)
		assert.Equal(t, node, &conf.MapNode{Data: map[string]conf.Node{
			"a": &conf.ValueNode{Data: "3"},
		}})
	})

	t.Run("map #2", func(t *testing.T) {
		node, err := conf.Parse(map[string]interface{}{"a.b": 3})
		assert.Nil(t, err)
		assert.Equal(t, node, &conf.MapNode{Data: map[string]conf.Node{
			"a": &conf.MapNode{Data: map[string]conf.Node{
				"b": &conf.ValueNode{Data: "3"},
			}},
		}})
	})

	t.Run("map #3", func(t *testing.T) {
		node, err := conf.Parse(map[string]interface{}{
			"a": map[string]interface{}{
				"b": 3,
			},
		})
		assert.Nil(t, err)
		assert.Equal(t, node, &conf.MapNode{Data: map[string]conf.Node{
			"a": &conf.MapNode{Data: map[string]conf.Node{
				"b": &conf.ValueNode{Data: "3"},
			}},
		}})
	})

	t.Run("map slice #1", func(t *testing.T) {
		node, err := conf.Parse(map[string]interface{}{"a": []int{3}})
		assert.Nil(t, err)
		assert.Equal(t, node, &conf.MapNode{Data: map[string]conf.Node{
			"a": &conf.ArrayNode{Data: []conf.Node{
				&conf.ValueNode{Data: "3"},
			}},
		}})
	})

	t.Run("map slice #2", func(t *testing.T) {
		node, err := conf.Parse(map[string]interface{}{"a[1]": 3})
		assert.Nil(t, err)
		assert.Equal(t, node, &conf.MapNode{Data: map[string]conf.Node{
			"a": &conf.ArrayNode{Data: []conf.Node{
				nil,
				&conf.ValueNode{Data: "3"},
			}},
		}})
	})

	t.Run("map slice #3", func(t *testing.T) {
		node, err := conf.Parse(map[string]interface{}{"a.2": 3})
		assert.Nil(t, err)
		assert.Equal(t, node, &conf.MapNode{Data: map[string]conf.Node{
			"a": &conf.ArrayNode{Data: []conf.Node{
				nil,
				nil,
				&conf.ValueNode{Data: "3"},
			}},
		}})
	})
}

func TestKeyConflict(t *testing.T) {

	t.Run("conflicts key #1 value and slice", func(t *testing.T) {
		_, err := conf.Parse(map[string]interface{}{
			"a.b":    3,
			"a.b[0]": 3,
		})
		assert.Error(t, err, "conf a.b\\[0]:\"3\" conflicts with a.b:\"3\"")
	})

	t.Run("conflicts key #2 value and slice", func(t *testing.T) {
		_, err := conf.Parse(map[string]interface{}{
			"a.b": 3,
			"a": map[string]interface{}{
				"b[0]": 3,
			},
		})
		assert.Error(t, err, "conf a.b:\"3\" conflicts with a.b:\\[\"3\"]")
	})

	t.Run("conflicts key #3 value and slice", func(t *testing.T) {
		_, err := conf.Parse(map[string]interface{}{
			"a.b": 3,
			"a": map[string]interface{}{
				"b": []int{3},
			},
		})
		assert.Error(t, err, "conf a.b:\"3\" conflicts with a.b:\\[\"3\"]")
	})

	t.Run("conflicts key #4 value and slice", func(t *testing.T) {
		_, err := conf.Parse(map[string]interface{}{
			"a.b": 3,
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"0": 3,
				},
			},
		})
		assert.Error(t, err, "conf a.b:\"3\" conflicts with a.b:\\[\"3\"]")
	})

	t.Run("conflicts key #1 value and map", func(t *testing.T) {
		_, err := conf.Parse(map[string]interface{}{
			"a.b":   3,
			"a.b.c": 3,
		})
		assert.Error(t, err, "conf a.b.c:\"3\" conflicts with a.b:\"3\"")
	})

	t.Run("conflicts key #2 value and map", func(t *testing.T) {
		_, err := conf.Parse(map[string]interface{}{
			"a.b": 3,
			"a": map[string]interface{}{
				"b.c": 3,
			},
		})
		assert.Error(t, err, "conf a.b:\"3\" conflicts with a.b:{\"c\":\"3\"}")
	})

	t.Run("conflicts key #3 value and map", func(t *testing.T) {
		_, err := conf.Parse(map[string]interface{}{
			"a.b": 3,
			"a": map[string]interface{}{
				"b": map[string]int{
					"c": 3,
				},
			},
		})
		assert.Error(t, err, "conf a.b:\"3\" conflicts with a.b:{\"c\":\"3\"}")
	})

	t.Run("conflicts key #1 map and slice", func(t *testing.T) {
		_, err := conf.Parse(map[string]interface{}{
			"a.b.c":  3,
			"a.b[0]": 3,
		})
		assert.Error(t, err, "conf a.b\\[0]:\"3\" conflicts with a.b:{\"c\":\"3\"}")
	})

	t.Run("conflicts key #2 map and slice", func(t *testing.T) {
		_, err := conf.Parse(map[string]interface{}{
			"a.b": map[string]int{
				"c": 3,
			},
			"a": map[string]interface{}{
				"b": []int{3},
			},
		})
		assert.Error(t, err, "conf a.b:{\"c\":\"3\"} conflicts with a.b:\\[\"3\"]")
	})

	t.Run("conflicts key #3 map and slice", func(t *testing.T) {
		_, err := conf.Parse(map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c": 3,
				},
			},
			"a.b": map[string]interface{}{
				"0": 3,
			},
		})
		assert.Error(t, err, "conf a.b:\\[\"3\"] conflicts with a.b:{\"c\":\"3\"}")
	})
}
