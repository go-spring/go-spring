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

package cast_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cast"
)

func TestNodeClone(t *testing.T) {
	node := &cast.MapNode{Data: map[string]cast.Node{
		"nil.in.map": &cast.NilNode{},
		"val.in.map": &cast.ValueNode{Data: "val.in.map"},
		"map.in.map": &cast.MapNode{Data: map[string]cast.Node{
			"nil.in.map":         &cast.NilNode{},
			"val.in.map":         &cast.ValueNode{Data: "val.in.map"},
			"empty.map.in.map":   &cast.MapNode{Data: map[string]cast.Node{}},
			"empty.array.in.map": &cast.ArrayNode{Data: []cast.Node{}},
		}},
		"array.in.map": &cast.ArrayNode{Data: []cast.Node{
			/* nil.in.array */ &cast.NilNode{},
			/* value.in.array */ &cast.ValueNode{Data: "value.in.array"},
			/* empty.map.in.array */ &cast.MapNode{Data: map[string]cast.Node{}},
			/* empty.array.in.array */ &cast.ArrayNode{Data: []cast.Node{}},
			/* map.in.array */ &cast.MapNode{Data: map[string]cast.Node{
				"nil.in.map": &cast.NilNode{},
				"val.in.map": &cast.ValueNode{Data: "val.in.map"},
				"map.in.map": &cast.MapNode{Data: map[string]cast.Node{
					"nil.in.map":         &cast.NilNode{},
					"val.in.map":         &cast.ValueNode{Data: "val.in.map"},
					"empty.map.in.map":   &cast.MapNode{Data: map[string]cast.Node{}},
					"empty.array.in.map": &cast.ArrayNode{Data: []cast.Node{}},
				}},
			}},
			/* array.in.array */ &cast.ArrayNode{Data: []cast.Node{
				/* nil.in.array */ &cast.NilNode{},
				/* value.in.array */ &cast.ValueNode{Data: "value.in.array"},
				/* empty.map.in.array */ &cast.MapNode{Data: map[string]cast.Node{}},
				/* empty.array.in.array */ &cast.ArrayNode{Data: []cast.Node{}},
			}},
		}},
	}}
	assert.Equal(t, node.Clone(), node)
}

func TestNodeValue(t *testing.T) {
	node := &cast.MapNode{Data: map[string]cast.Node{
		"nil.in.map": &cast.NilNode{},
		"val.in.map": &cast.ValueNode{Data: "val.in.map"},
		"map.in.map": &cast.MapNode{Data: map[string]cast.Node{
			"nil.in.map":         &cast.NilNode{},
			"val.in.map":         &cast.ValueNode{Data: "val.in.map"},
			"empty.map.in.map":   &cast.MapNode{Data: map[string]cast.Node{}},
			"empty.array.in.map": &cast.ArrayNode{Data: []cast.Node{}},
		}},
		"array.in.map": &cast.ArrayNode{Data: []cast.Node{
			/* nil.in.array */ &cast.NilNode{},
			/* value.in.array */ &cast.ValueNode{Data: "value.in.array"},
			/* empty.map.in.array */ &cast.MapNode{Data: map[string]cast.Node{}},
			/* empty.array.in.array */ &cast.ArrayNode{Data: []cast.Node{}},
			/* map.in.array */ &cast.MapNode{Data: map[string]cast.Node{
				"nil.in.map": &cast.NilNode{},
				"val.in.map": &cast.ValueNode{Data: "val.in.map"},
				"map.in.map": &cast.MapNode{Data: map[string]cast.Node{
					"nil.in.map":         &cast.NilNode{},
					"val.in.map":         &cast.ValueNode{Data: "val.in.map"},
					"empty.map.in.map":   &cast.MapNode{Data: map[string]cast.Node{}},
					"empty.array.in.map": &cast.ArrayNode{Data: []cast.Node{}},
				}},
			}},
			/* array.in.array */ &cast.ArrayNode{Data: []cast.Node{
				/* nil.in.array */ &cast.NilNode{},
				/* value.in.array */ &cast.ValueNode{Data: "value.in.array"},
				/* empty.map.in.array */ &cast.MapNode{Data: map[string]cast.Node{}},
				/* empty.array.in.array */ &cast.ArrayNode{Data: []cast.Node{}},
			}},
		}},
	}}
	assert.Equal(t, node.Value(), map[string]interface{}{
		"nil.in.map": nil,
		"val.in.map": "val.in.map",
		"map.in.map": map[string]interface{}{
			"nil.in.map":         nil,
			"val.in.map":         "val.in.map",
			"empty.map.in.map":   map[string]interface{}{},
			"empty.array.in.map": []interface{}{},
		},
		"array.in.map": []interface{}{
			/* nil.in.array */ nil,
			/* value.in.array */ "value.in.array",
			/* empty.map.in.array */ map[string]interface{}{},
			/* empty.array.in.array */ []interface{}{},
			/* map.in.array */ map[string]interface{}{
				"nil.in.map": nil,
				"val.in.map": "val.in.map",
				"map.in.map": map[string]interface{}{
					"nil.in.map":         nil,
					"val.in.map":         "val.in.map",
					"empty.map.in.map":   map[string]interface{}{},
					"empty.array.in.map": []interface{}{},
				},
			},
			/* array.in.array */ []interface{}{
				/* nil.in.array */ nil,
				/* value.in.array */ "value.in.array",
				/* empty.map.in.array */ map[string]interface{}{},
				/* empty.array.in.array */ []interface{}{},
			},
		},
	})
}

func TestNodeJSON(t *testing.T) {
	node := &cast.MapNode{Data: map[string]cast.Node{
		"nil.in.map": &cast.NilNode{},
		"val.in.map": &cast.ValueNode{Data: "val.in.map"},
		"map.in.map": &cast.MapNode{Data: map[string]cast.Node{
			"nil.in.map":         &cast.NilNode{},
			"val.in.map":         &cast.ValueNode{Data: "val.in.map"},
			"empty.map.in.map":   &cast.MapNode{Data: map[string]cast.Node{}},
			"empty.array.in.map": &cast.ArrayNode{Data: []cast.Node{}},
		}},
		"array.in.map": &cast.ArrayNode{Data: []cast.Node{
			/* nil.in.array */ &cast.NilNode{},
			/* value.in.array */ &cast.ValueNode{Data: "value.in.array"},
			/* empty.map.in.array */ &cast.MapNode{Data: map[string]cast.Node{}},
			/* empty.array.in.array */ &cast.ArrayNode{Data: []cast.Node{}},
			/* map.in.array */ &cast.MapNode{Data: map[string]cast.Node{
				"nil.in.map": &cast.NilNode{},
				"val.in.map": &cast.ValueNode{Data: "val.in.map"},
				"map.in.map": &cast.MapNode{Data: map[string]cast.Node{
					"nil.in.map":         &cast.NilNode{},
					"val.in.map":         &cast.ValueNode{Data: "val.in.map"},
					"empty.map.in.map":   &cast.MapNode{Data: map[string]cast.Node{}},
					"empty.array.in.map": &cast.ArrayNode{Data: []cast.Node{}},
				}},
			}},
			/* array.in.array */ &cast.ArrayNode{Data: []cast.Node{
				/* nil.in.array */ &cast.NilNode{},
				/* value.in.array */ &cast.ValueNode{Data: "value.in.array"},
				/* empty.map.in.array */ &cast.MapNode{Data: map[string]cast.Node{}},
				/* empty.array.in.array */ &cast.ArrayNode{Data: []cast.Node{}},
			}},
		}},
	}}
	assert.JsonEqual(t, node.JSON(), `{
		"nil.in.map": null,
		"val.in.map": "val.in.map",
		"map.in.map": {
			"nil.in.map": null,
			"val.in.map": "val.in.map",
			"empty.map.in.map": {},
			"empty.array.in.map": []
		},
		"array.in.map": [
			null,
			"value.in.array",
			{},
			[],
			{
				"nil.in.map": null,
				"val.in.map": "val.in.map",
				"map.in.map": {
					"nil.in.map": null,
					"val.in.map": "val.in.map",
					"empty.map.in.map": {},
					"empty.array.in.map": []
				}
			},
			[
				null,
				"value.in.array",
				{},
				[]
			]
		]
	}`)
}

func TestFlatNode(t *testing.T) {
	node := &cast.MapNode{Data: map[string]cast.Node{
		"nil.in.map": &cast.NilNode{},
		"val.in.map": &cast.ValueNode{Data: "val.in.map"},
		"map.in.map": &cast.MapNode{Data: map[string]cast.Node{
			"nil.in.map":         &cast.NilNode{},
			"val.in.map":         &cast.ValueNode{Data: "val.in.map"},
			"empty.map.in.map":   &cast.MapNode{Data: map[string]cast.Node{}},
			"empty.array.in.map": &cast.ArrayNode{Data: []cast.Node{}},
		}},
		"array.in.map": &cast.ArrayNode{Data: []cast.Node{
			/* nil.in.array */ &cast.NilNode{},
			/* value.in.array */ &cast.ValueNode{Data: "value.in.array"},
			/* empty.map.in.array */ &cast.MapNode{Data: map[string]cast.Node{}},
			/* empty.array.in.array */ &cast.ArrayNode{Data: []cast.Node{}},
			/* map.in.array */ &cast.MapNode{Data: map[string]cast.Node{
				"nil.in.map": &cast.NilNode{},
				"val.in.map": &cast.ValueNode{Data: "val.in.map"},
				"map.in.map": &cast.MapNode{Data: map[string]cast.Node{
					"nil.in.map":         &cast.NilNode{},
					"val.in.map":         &cast.ValueNode{Data: "val.in.map"},
					"empty.map.in.map":   &cast.MapNode{Data: map[string]cast.Node{}},
					"empty.array.in.map": &cast.ArrayNode{Data: []cast.Node{}},
				}},
			}},
			/* array.in.array */ &cast.ArrayNode{Data: []cast.Node{
				/* nil.in.array */ &cast.NilNode{},
				/* value.in.array */ &cast.ValueNode{Data: "value.in.array"},
				/* empty.map.in.array */ &cast.MapNode{Data: map[string]cast.Node{}},
				/* empty.array.in.array */ &cast.ArrayNode{Data: []cast.Node{}},
			}},
		}},
	}}
	fmt.Printf("%#v\n", cast.FlatNode(node))
	assert.Equal(t, cast.FlatNode(node), map[string]string{
		"$.[array.in.map][0]": "<nil>",
		"$.[array.in.map][1]": "value.in.array",
		"$.[array.in.map][2]": "{}",
		"$.[array.in.map][3]": "[]",
		"$.[array.in.map][4].[map.in.map].[empty.array.in.map]": "[]",
		"$.[array.in.map][4].[map.in.map].[empty.map.in.map]":   "{}",
		"$.[array.in.map][4].[map.in.map].[nil.in.map]":         "<nil>",
		"$.[array.in.map][4].[map.in.map].[val.in.map]":         "val.in.map",
		"$.[array.in.map][4].[nil.in.map]":                      "<nil>",
		"$.[array.in.map][4].[val.in.map]":                      "val.in.map",
		"$.[array.in.map][5][0]":                                "<nil>",
		"$.[array.in.map][5][1]":                                "value.in.array",
		"$.[array.in.map][5][2]":                                "{}",
		"$.[array.in.map][5][3]":                                "[]",
		"$.[map.in.map].[empty.array.in.map]":                   "[]",
		"$.[map.in.map].[empty.map.in.map]":                     "{}",
		"$.[map.in.map].[nil.in.map]":                           "<nil>",
		"$.[map.in.map].[val.in.map]":                           "val.in.map",
		"$.[nil.in.map]":                                        "<nil>",
		"$.[val.in.map]":                                        "val.in.map",
	})
}
