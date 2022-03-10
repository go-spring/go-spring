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
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cast"
)

func printResult(t *testing.T, m map[string]string) {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(k, "=", string(b))
	}
}

func TestFlatJSON_String(t *testing.T) {

	var testcases = []struct {
		data   string
		expect map[string]string
	}{
		{
			data:   `null`,
			expect: map[string]string{`$`: `null`},
		},
		{
			data:   `3`,
			expect: map[string]string{`$`: `3`},
		},
		{
			data:   `"3"`,
			expect: map[string]string{`$`: `"3"`},
		},
		{
			data:   `"\"3\""`,
			expect: map[string]string{`$.""`: `"3"`},
		},
		{
			data:   `"\"\\\"3\\\"\""`,
			expect: map[string]string{`$."".""`: `"3"`},
		},
		{
			data:   `true`,
			expect: map[string]string{`$`: `true`},
		},
		{
			data:   `"true"`,
			expect: map[string]string{`$`: `"true"`},
		},
		{
			data:   `"\"true\""`,
			expect: map[string]string{`$.""`: `"true"`},
		},
		{
			data:   `"\"\\\"true\\\"\""`,
			expect: map[string]string{`$."".""`: `"true"`},
		},
		{
			data:   `abc`,
			expect: map[string]string{`$`: `abc`},
		},
		{
			data:   `"abc"`,
			expect: map[string]string{`$`: `"abc"`},
		},
		{
			data:   `"\"abc\""`,
			expect: map[string]string{`$.""`: `"abc"`},
		},
		{
			data:   `"\"\\\"abc\\\"\""`,
			expect: map[string]string{`$."".""`: `"abc"`},
		},
		{
			data:   `{`,
			expect: map[string]string{`$`: `{`},
		},
		{
			data:   `"{"`,
			expect: map[string]string{`$`: `"{"`},
		},
		{
			data:   `"\"{\""`,
			expect: map[string]string{`$.""`: `"{"`},
		},
		{
			data:   `"\"\\\"{\\\"\""`,
			expect: map[string]string{`$."".""`: `"{"`},
		},
		{
			data:   `}`,
			expect: map[string]string{`$`: `}`},
		},
		{
			data:   `"}"`,
			expect: map[string]string{`$`: `"}"`},
		},
		{
			data:   `"\"}\""`,
			expect: map[string]string{`$.""`: `"}"`},
		},
		{
			data:   `"\"\\\"}\\\"\""`,
			expect: map[string]string{`$."".""`: `"}"`},
		},
		{
			data:   `{}`,
			expect: map[string]string{`$`: `{}`},
		},
		{
			data:   `"{}"`,
			expect: map[string]string{`$.""`: `{}`},
		},
		{
			data:   `"\"{}\""`,
			expect: map[string]string{`$."".""`: `{}`},
		},
		{
			data:   `[`,
			expect: map[string]string{`$`: `[`},
		},
		{
			data:   `"["`,
			expect: map[string]string{`$`: `"["`},
		},
		{
			data:   `"\"[\""`,
			expect: map[string]string{`$.""`: `"["`},
		},
		{
			data:   `"\"\\\"[\\\"\""`,
			expect: map[string]string{`$."".""`: `"["`},
		},
		{
			data:   `]`,
			expect: map[string]string{`$`: `]`},
		},
		{
			data:   `"]"`,
			expect: map[string]string{`$`: `"]"`},
		},
		{
			data:   `"\"]\""`,
			expect: map[string]string{`$.""`: `"]"`},
		},
		{
			data:   `"\"\\\"]\\\"\""`,
			expect: map[string]string{`$."".""`: `"]"`},
		},
		{
			data:   `[]`,
			expect: map[string]string{`$`: `[]`},
		},
		{
			data:   `"[]"`,
			expect: map[string]string{`$.""`: `[]`},
		},
		{
			data:   `"\"[]\""`,
			expect: map[string]string{`$."".""`: `[]`},
		},
		{
			data:   `{"a":null}`,
			expect: map[string]string{`$.a`: `null`},
		},
		{
			data:   `{"a":3}`,
			expect: map[string]string{`$.a`: `3`},
		},
		{
			data:   `{"a":"3"}`,
			expect: map[string]string{`$.a`: `"3"`},
		},
		{
			data:   `{"a":"\"3\""}`,
			expect: map[string]string{`$.a.""`: `"3"`},
		},
		{
			data:   `{"a":true}`,
			expect: map[string]string{`$.a`: `true`},
		},
		{
			data:   `{"a":"true"}`,
			expect: map[string]string{`$.a`: `"true"`},
		},
		{
			data:   `{"a":"\"true\""}`,
			expect: map[string]string{`$.a.""`: `"true"`},
		},
		{
			data:   `{"a":b}`,
			expect: map[string]string{`$`: `{"a":b}`},
		},
		{
			data:   `{"a":"b"}`,
			expect: map[string]string{`$.a`: `"b"`},
		},
		{
			data:   `{"a":"\"b\""}`,
			expect: map[string]string{`$.a.""`: `"b"`},
		},
		{
			data:   `{"a":{}`,
			expect: map[string]string{`$`: `{"a":{}`},
		},
		{
			data:   `{"a":"{"}`,
			expect: map[string]string{`$.a`: `"{"`},
		},
		{
			data:   `{"a":"\"{\""}`,
			expect: map[string]string{`$.a.""`: `"{"`},
		},
		{
			data:   `{"a":}}`,
			expect: map[string]string{`$`: `{"a":}}`},
		},
		{
			data:   `{"a":"}"}`,
			expect: map[string]string{`$.a`: `"}"`},
		},
		{
			data:   `{"a":"\"}\""}`,
			expect: map[string]string{`$.a.""`: `"}"`},
		},
		{
			data:   `{"a":{}}`,
			expect: map[string]string{`$.a`: `{}`},
		},
		{
			data:   `{"a":"{}"}`,
			expect: map[string]string{`$.a.""`: `{}`},
		},
		{
			data:   `{"a":[}`,
			expect: map[string]string{`$`: `{"a":[}`},
		},
		{
			data:   `{"a":"["}`,
			expect: map[string]string{`$.a`: `"["`},
		},
		{
			data:   `{"a":"\"[\""}`,
			expect: map[string]string{`$.a.""`: `"["`},
		},
		{
			data:   `{"a":]}`,
			expect: map[string]string{`$`: `{"a":]}`},
		},
		{
			data:   `{"a":"]"}`,
			expect: map[string]string{`$.a`: `"]"`},
		},
		{
			data:   `{"a":"\"]\""}`,
			expect: map[string]string{`$.a.""`: `"]"`},
		},
		{
			data:   `{"a":[]}`,
			expect: map[string]string{`$.a`: `[]`},
		},
		{
			data:   `{"a":"[]"}`,
			expect: map[string]string{`$.a.""`: `[]`},
		},
		{
			data:   `[3,"3","\"3\""]`,
			expect: map[string]string{`$[0]`: `3`, `$[1]`: `"3"`, `$[2].""`: `"3"`},
		},
		{
			data:   `[true,"true","\"true\""]`,
			expect: map[string]string{`$[0]`: `true`, `$[1]`: `"true"`, `$[2].""`: `"true"`},
		},
		{
			data:   `[null,"null","\"null\""]`,
			expect: map[string]string{`$[0]`: `null`, `$[1]`: `"null"`, `$[2].""`: `"null"`},
		},
		{
			data:   `[a]`,
			expect: map[string]string{`$`: `[a]`},
		},
		{
			data:   `["a"]`,
			expect: map[string]string{`$[0]`: `"a"`},
		},
		{
			data:   `[{},"{}","\"{}\""]`,
			expect: map[string]string{`$[0]`: `{}`, `$[1].""`: `{}`, `$[2]."".""`: `{}`},
		},
		{
			data:   `[[],"[]","\"[]\""]`,
			expect: map[string]string{`$[0]`: `[]`, `$[1].""`: `[]`, `$[2]."".""`: `[]`},
		},
	}

	for _, c := range testcases {
		m := cast.FlatJSON(c.data)
		printResult(t, m)
		assert.Equal(t, m, c.expect)
	}
}

func TestFlatJSON_StringSlice(t *testing.T) {

	var testcases = []struct {
		data   []string
		expect map[string]string
	}{
		{
			data:   []string{`null`},
			expect: map[string]string{`$[0]`: `null`},
		},
		{
			data:   []string{`3`},
			expect: map[string]string{`$[0]`: `3`},
		},
		{
			data:   []string{`"3"`},
			expect: map[string]string{`$[0]`: `"3"`},
		},
		{
			data:   []string{`"\"3\""`},
			expect: map[string]string{`$[0].""`: `"3"`},
		},
		{
			data:   []string{`"\"\\\"3\\\"\""`},
			expect: map[string]string{`$[0]."".""`: `"3"`},
		},
		{
			data:   []string{`true`},
			expect: map[string]string{`$[0]`: `true`},
		},
		{
			data:   []string{`"true"`},
			expect: map[string]string{`$[0]`: `"true"`},
		},
		{
			data:   []string{`"\"true\""`},
			expect: map[string]string{`$[0].""`: `"true"`},
		},
		{
			data:   []string{`"\"\\\"true\\\"\""`},
			expect: map[string]string{`$[0]."".""`: `"true"`},
		},
		{
			data:   []string{`abc`},
			expect: map[string]string{`$[0]`: `abc`},
		},
		{
			data:   []string{`"abc"`},
			expect: map[string]string{`$[0]`: `"abc"`},
		},
		{
			data:   []string{`"\"abc\""`},
			expect: map[string]string{`$[0].""`: `"abc"`},
		},
		{
			data:   []string{`"\"\\\"abc\\\"\""`},
			expect: map[string]string{`$[0]."".""`: `"abc"`},
		},
		{
			data:   []string{`{`},
			expect: map[string]string{`$[0]`: `{`},
		},
		{
			data:   []string{`"{"`},
			expect: map[string]string{`$[0]`: `"{"`},
		},
		{
			data:   []string{`"\"{\""`},
			expect: map[string]string{`$[0].""`: `"{"`},
		},
		{
			data:   []string{`"\"\\\"{\\\"\""`},
			expect: map[string]string{`$[0]."".""`: `"{"`},
		},
		{
			data:   []string{`}`},
			expect: map[string]string{`$[0]`: `}`},
		},
		{
			data:   []string{`"}"`},
			expect: map[string]string{`$[0]`: `"}"`},
		},
		{
			data:   []string{`"\"}\""`},
			expect: map[string]string{`$[0].""`: `"}"`},
		},
		{
			data:   []string{`"\"\\\"}\\\"\""`},
			expect: map[string]string{`$[0]."".""`: `"}"`},
		},
		{
			data:   []string{`{}`},
			expect: map[string]string{`$[0]`: `{}`},
		},
		{
			data:   []string{`"{}"`},
			expect: map[string]string{`$[0].""`: `{}`},
		},
		{
			data:   []string{`"\"{}\""`},
			expect: map[string]string{`$[0]."".""`: `{}`},
		},
		{
			data:   []string{`[`},
			expect: map[string]string{`$[0]`: `[`},
		},
		{
			data:   []string{`"["`},
			expect: map[string]string{`$[0]`: `"["`},
		},
		{
			data:   []string{`"\"[\""`},
			expect: map[string]string{`$[0].""`: `"["`},
		},
		{
			data:   []string{`"\"\\\"[\\\"\""`},
			expect: map[string]string{`$[0]."".""`: `"["`},
		},
		{
			data:   []string{`]`},
			expect: map[string]string{`$[0]`: `]`},
		},
		{
			data:   []string{`"]"`},
			expect: map[string]string{`$[0]`: `"]"`},
		},
		{
			data:   []string{`"\"]\""`},
			expect: map[string]string{`$[0].""`: `"]"`},
		},
		{
			data:   []string{`"\"\\\"]\\\"\""`},
			expect: map[string]string{`$[0]."".""`: `"]"`},
		},
		{
			data:   []string{`[]`},
			expect: map[string]string{`$[0]`: `[]`},
		},
		{
			data:   []string{`"[]"`},
			expect: map[string]string{`$[0].""`: `[]`},
		},
		{
			data:   []string{`"\"[]\""`},
			expect: map[string]string{`$[0]."".""`: `[]`},
		},
		{
			data:   []string{`{"a":null}`},
			expect: map[string]string{`$[0].a`: `null`},
		},
		{
			data:   []string{`{"a":3}`},
			expect: map[string]string{`$[0].a`: `3`},
		},
		{
			data:   []string{`{"a":"3"}`},
			expect: map[string]string{`$[0].a`: `"3"`},
		},
		{
			data:   []string{`{"a":"\"3\""}`},
			expect: map[string]string{`$[0].a.""`: `"3"`},
		},
		{
			data:   []string{`{"a":true}`},
			expect: map[string]string{`$[0].a`: `true`},
		},
		{
			data:   []string{`{"a":"true"}`},
			expect: map[string]string{`$[0].a`: `"true"`},
		},
		{
			data:   []string{`{"a":"\"true\""}`},
			expect: map[string]string{`$[0].a.""`: `"true"`},
		},
		{
			data:   []string{`{"a":b}`},
			expect: map[string]string{`$[0]`: `{"a":b}`},
		},
		{
			data:   []string{`{"a":"b"}`},
			expect: map[string]string{`$[0].a`: `"b"`},
		},
		{
			data:   []string{`{"a":"\"b\""}`},
			expect: map[string]string{`$[0].a.""`: `"b"`},
		},
		{
			data:   []string{`{"a":{}`},
			expect: map[string]string{`$[0]`: `{"a":{}`},
		},
		{
			data:   []string{`{"a":"{"}`},
			expect: map[string]string{`$[0].a`: `"{"`},
		},
		{
			data:   []string{`{"a":"\"{\""}`},
			expect: map[string]string{`$[0].a.""`: `"{"`},
		},
		{
			data:   []string{`{"a":}}`},
			expect: map[string]string{`$[0]`: `{"a":}}`},
		},
		{
			data:   []string{`{"a":"}"}`},
			expect: map[string]string{`$[0].a`: `"}"`},
		},
		{
			data:   []string{`{"a":"\"}\""}`},
			expect: map[string]string{`$[0].a.""`: `"}"`},
		},
		{
			data:   []string{`{"a":{}}`},
			expect: map[string]string{`$[0].a`: `{}`},
		},
		{
			data:   []string{`{"a":"{}"}`},
			expect: map[string]string{`$[0].a.""`: `{}`},
		},
		{
			data:   []string{`{"a":[}`},
			expect: map[string]string{`$[0]`: `{"a":[}`},
		},
		{
			data:   []string{`{"a":"["}`},
			expect: map[string]string{`$[0].a`: `"["`},
		},
		{
			data:   []string{`{"a":"\"[\""}`},
			expect: map[string]string{`$[0].a.""`: `"["`},
		},
		{
			data:   []string{`{"a":]}`},
			expect: map[string]string{`$[0]`: `{"a":]}`},
		},
		{
			data:   []string{`{"a":"]"}`},
			expect: map[string]string{`$[0].a`: `"]"`},
		},
		{
			data:   []string{`{"a":"\"]\""}`},
			expect: map[string]string{`$[0].a.""`: `"]"`},
		},
		{
			data:   []string{`{"a":[]}`},
			expect: map[string]string{`$[0].a`: `[]`},
		},
		{
			data:   []string{`{"a":"[]"}`},
			expect: map[string]string{`$[0].a.""`: `[]`},
		},
		{
			data:   []string{`[3,"3","\"3\""]`},
			expect: map[string]string{`$[0][0]`: `3`, `$[0][1]`: `"3"`, `$[0][2].""`: `"3"`},
		},
		{
			data:   []string{`[true,"true","\"true\""]`},
			expect: map[string]string{`$[0][0]`: `true`, `$[0][1]`: `"true"`, `$[0][2].""`: `"true"`},
		},
		{
			data:   []string{`[null,"null","\"null\""]`},
			expect: map[string]string{`$[0][0]`: `null`, `$[0][1]`: `"null"`, `$[0][2].""`: `"null"`},
		},
		{
			data:   []string{`[a]`},
			expect: map[string]string{`$[0]`: `[a]`},
		},
		{
			data:   []string{`["a"]`},
			expect: map[string]string{`$[0][0]`: `"a"`},
		},
		{
			data:   []string{`[{},"{}","\"{}\""]`},
			expect: map[string]string{`$[0][0]`: `{}`, `$[0][1].""`: `{}`, `$[0][2]."".""`: `{}`},
		},
		{
			data:   []string{`[[],"[]","\"[]\""]`},
			expect: map[string]string{`$[0][0]`: `[]`, `$[0][1].""`: `[]`, `$[0][2]."".""`: `[]`},
		},
	}

	for _, c := range testcases {
		m := cast.FlatJSON(c.data)
		printResult(t, m)
		assert.Equal(t, m, c.expect)
	}
}
