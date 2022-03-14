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
	"fmt"
	"reflect"
	"testing"

	"github.com/go-spring/spring-base/internal/conf"
)

func TestFlat(t *testing.T) {

	testcases := []struct {
		value  interface{}
		expect map[string]string
	}{
		{
			nil,
			map[string]string{
				"$": "<nil>",
			},
		},
		{
			true,
			map[string]string{
				"$": "true",
			},
		},
		{
			3,
			map[string]string{
				"$": "3",
			},
		},
		{
			3.0,
			map[string]string{
				"$": "3",
			},
		},
		{
			3.1,
			map[string]string{
				"$": "3.1",
			},
		},
		{
			"3",
			map[string]string{
				"$": "3",
			},
		},
		{
			[]bool{},
			map[string]string{
				"$": "[]",
			},
		},
		{
			map[string]interface{}{},
			map[string]string{
				"$": "{}",
			},
		},
		{
			[]bool{true, false},
			map[string]string{
				"$[0]": "true",
				"$[1]": "false",
			},
		},
		{
			map[int]interface{}{
				0: nil,
				2: false,
			},
			map[string]string{
				"$[0]": "<nil>",
				"$[2]": "false",
			},
		},
		{
			map[string]interface{}{
				"a":   nil,
				"b.c": false,
			},
			map[string]string{
				"$.a":   "<nil>",
				"$.b.c": "false",
			},
		},
	}

	for i, c := range testcases {
		var node conf.Node
		err := conf.Parse(&node, c.value)
		if err != nil {
			t.Fatal(err)
		}
		result := conf.Flat(node)
		fmt.Println(result)
		if !reflect.DeepEqual(result, c.expect) {
			t.Fatalf("%d: got %#v but expect %#v", i, result, c.expect)
		}
	}
}
