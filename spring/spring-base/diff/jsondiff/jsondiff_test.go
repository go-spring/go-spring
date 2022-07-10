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

package jsondiff_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/diff/jsondiff"
)

func TestDiff(t *testing.T) {

	t.Run("Equal & UnquoteExpand", func(t *testing.T) {

		testcases := []struct {
			A string
			B string
			C []*jsondiff.Config
			R *jsondiff.DiffResult
		}{
			{
				A: `null`,
				B: `null`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						"$": {A: "null", B: "null"},
					},
				},
			},
			{
				A: `3`,
				B: `3`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						"$": {A: "3", B: "3"},
					},
				},
			},
			{
				A: `"3"`,
				B: `"3"`,
				C: []*jsondiff.Config{
					jsondiff.Path("$").UnquoteExpand(),
				},
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[""]`: {A: "3", B: "3"},
					},
				},
			},
			{
				A: `true`,
				B: `true`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						"$": {A: "true", B: "true"},
					},
				},
			},
			{
				A: `"true"`,
				B: `"true"`,
				C: []*jsondiff.Config{
					jsondiff.Path("$").UnquoteExpand(),
				},
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[""]`: {A: "true", B: "true"},
					},
				},
			},
			{
				A: `abc`,
				B: `abc`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						"$": {A: "abc", B: "abc"},
					},
				},
			},
			{
				A: `"abc"`,
				B: `"abc"`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$`: {A: `"abc"`, B: `"abc"`},
					},
				},
			},
			{
				A: `{}`,
				B: `{}`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$`: {A: "{}", B: "{}"},
					},
				},
			},
			{
				A: `"{}"`,
				B: `"{}"`,
				C: []*jsondiff.Config{
					jsondiff.Path("$").UnquoteExpand(),
				},
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[""]`: {A: "{}", B: "{}"},
					},
				},
			},
			{
				A: `[]`,
				B: `[]`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$`: {A: "[]", B: "[]"},
					},
				},
			},
			{
				A: `"[]"`,
				B: `"[]"`,
				C: []*jsondiff.Config{
					jsondiff.Path("$").UnquoteExpand(),
				},
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[""]`: {A: "[]", B: "[]"},
					},
				},
			},
			{
				A: `{"a":null}`,
				B: `{"a":null}`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: "null", B: "null"},
					},
				},
			},
			{
				A: `{"a":3}`,
				B: `{"a":3}`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: "3", B: "3"},
					},
				},
			},
			{
				A: `{"a":"3"}`,
				B: `{"a":"3"}`,
				C: []*jsondiff.Config{
					jsondiff.Path("$[a]").UnquoteExpand(),
				},
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a][""]`: {A: "3", B: "3"},
					},
				},
			},
			{
				A: `{"a":true}`,
				B: `{"a":true}`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: "true", B: "true"},
					},
				},
			},
			{
				A: `{"a":"true"}`,
				B: `{"a":"true"}`,
				C: []*jsondiff.Config{
					jsondiff.Path("$[a]").UnquoteExpand(),
				},
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a][""]`: {A: "true", B: "true"},
					},
				},
			},
			{
				A: `{"a":"b"}`,
				B: `{"a":"b"}`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: `"b"`, B: `"b"`},
					},
				},
			},
			{
				A: `{"a":{}}`,
				B: `{"a":{}}`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: "{}", B: "{}"},
					},
				},
			},
			{
				A: `{"a":"{}"}`,
				B: `{"a":"{}"}`,
				C: []*jsondiff.Config{
					jsondiff.Path("$[a]").UnquoteExpand(),
				},
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a][""]`: {A: "{}", B: "{}"},
					},
				},
			},
			{
				A: `{"a":[]}`,
				B: `{"a":[]}`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: "[]", B: "[]"},
					},
				},
			},
			{
				A: `{"a":"[]"}`,
				B: `{"a":"[]"}`,
				C: []*jsondiff.Config{
					jsondiff.Path("$[a]").UnquoteExpand(),
				},
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a][""]`: {A: "[]", B: "[]"},
					},
				},
			},
			{
				A: `[3,"3"]`,
				B: `[3,"3"]`,
				C: []*jsondiff.Config{
					jsondiff.Path("$[1]").UnquoteExpand(),
				},
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[0]`:     {A: "3", B: "3"},
						`$[1][""]`: {A: "3", B: "3"},
					},
				},
			},
			{
				A: `[true,"true"]`,
				B: `[true,"true"]`,
				C: []*jsondiff.Config{
					jsondiff.Path("$[1]").UnquoteExpand(),
				},
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[0]`:     {A: "true", B: "true"},
						`$[1][""]`: {A: "true", B: "true"},
					},
				},
			},
			{
				A: `[null,"null"]`,
				B: `[null,"null"]`,
				C: []*jsondiff.Config{
					jsondiff.Path("$[1]").UnquoteExpand(),
				},
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[0]`:     {A: "null", B: "null"},
						`$[1][""]`: {A: "null", B: "null"},
					},
				},
			},
			{
				A: `[a]`,
				B: `[a]`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$`: {A: "[a]", B: "[a]"},
					},
				},
			},
			{
				A: `["a"]`,
				B: `["a"]`,
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[0]`: {A: `"a"`, B: `"a"`},
					},
				},
			},
			{
				A: `[{},"{}"]`,
				B: `[{},"{}"]`,
				C: []*jsondiff.Config{
					jsondiff.Path("$[1]").UnquoteExpand(),
				},
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[0]`:     {A: "{}", B: "{}"},
						`$[1][""]`: {A: "{}", B: "{}"},
					},
				},
			},
			{
				A: `[[],"[]"]`,
				B: `[[],"[]"]`,
				C: []*jsondiff.Config{
					jsondiff.Path("$[1]").UnquoteExpand(),
				},
				R: &jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[0]`:     {A: "[]", B: "[]"},
						`$[1][""]`: {A: "[]", B: "[]"},
					},
				},
			},
		}

		for i, c := range testcases {
			r := jsondiff.Diff(c.A, c.B, c.C...)
			assert.Equal(t, r, c.R, fmt.Sprintf("%d %s", i, c.A))
		}
	})

	t.Run("Equal & No UnquoteExpand", func(t *testing.T) {

		testcases := []struct {
			A string
			B string
			R *jsondiff.DiffResult
		}{
			{
				`null`,
				`null`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						"$": {A: "null", B: "null"},
					},
				},
			},
			{
				`3`,
				`3`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						"$": {A: "3", B: "3"},
					},
				},
			},
			{
				`"3"`,
				`"3"`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$`: {A: `"3"`, B: `"3"`},
					},
				},
			},
			{
				`true`,
				`true`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						"$": {A: "true", B: "true"},
					},
				},
			},
			{
				`"true"`,
				`"true"`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$`: {A: `"true"`, B: `"true"`},
					},
				},
			},
			{
				`abc`,
				`abc`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						"$": {A: "abc", B: "abc"},
					},
				},
			},
			{
				`"abc"`,
				`"abc"`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$`: {A: `"abc"`, B: `"abc"`},
					},
				},
			},
			{
				`{}`,
				`{}`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$`: {A: "{}", B: "{}"},
					},
				},
			},
			{
				`"{}"`,
				`"{}"`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$`: {A: `"{}"`, B: `"{}"`},
					},
				},
			},
			{
				`[]`,
				`[]`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$`: {A: "[]", B: "[]"},
					},
				},
			},
			{
				`"[]"`,
				`"[]"`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$`: {A: `"[]"`, B: `"[]"`},
					},
				},
			},
			{
				`{"a":null}`,
				`{"a":null}`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: "null", B: "null"},
					},
				},
			},
			{
				`{"a":3}`,
				`{"a":3}`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: "3", B: "3"},
					},
				},
			},
			{
				`{"a":"3"}`,
				`{"a":"3"}`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: `"3"`, B: `"3"`},
					},
				},
			},
			{
				`{"a":true}`,
				`{"a":true}`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: "true", B: "true"},
					},
				},
			},
			{
				`{"a":"true"}`,
				`{"a":"true"}`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: `"true"`, B: `"true"`},
					},
				},
			},
			{
				`{"a":"b"}`,
				`{"a":"b"}`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: `"b"`, B: `"b"`},
					},
				},
			},
			{
				`{"a":{}}`,
				`{"a":{}}`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: "{}", B: "{}"},
					},
				},
			},
			{
				`{"a":"{}"}`,
				`{"a":"{}"}`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: `"{}"`, B: `"{}"`},
					},
				},
			},
			{
				`{"a":[]}`,
				`{"a":[]}`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: "[]", B: "[]"},
					},
				},
			},
			{
				`{"a":"[]"}`,
				`{"a":"[]"}`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[a]`: {A: `"[]"`, B: `"[]"`},
					},
				},
			},
			{
				`[3,"3"]`,
				`[3,"3"]`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[0]`: {A: "3", B: "3"},
						`$[1]`: {A: `"3"`, B: `"3"`},
					},
				},
			},
			{
				`[true,"true"]`,
				`[true,"true"]`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[0]`: {A: "true", B: "true"},
						`$[1]`: {A: `"true"`, B: `"true"`},
					},
				},
			},
			{
				`[null,"null"]`,
				`[null,"null"]`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[0]`: {A: "null", B: "null"},
						`$[1]`: {A: `"null"`, B: `"null"`},
					},
				},
			},
			{
				`[a]`,
				`[a]`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$`: {A: "[a]", B: "[a]"},
					},
				},
			},
			{
				`["a"]`,
				`["a"]`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[0]`: {A: `"a"`, B: `"a"`},
					},
				},
			},
			{
				`[{},"{}"]`,
				`[{},"{}"]`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[0]`: {A: "{}", B: "{}"},
						`$[1]`: {A: `"{}"`, B: `"{}"`},
					},
				},
			},
			{
				`[[],"[]"]`,
				`[[],"[]"]`,
				&jsondiff.DiffResult{
					Differs: map[string]jsondiff.DiffItem{},
					Ignores: map[string]jsondiff.DiffItem{},
					Equals: map[string]jsondiff.DiffItem{
						`$[0]`: {A: "[]", B: "[]"},
						`$[1]`: {A: `"[]"`, B: `"[]"`},
					},
				},
			},
		}

		for i, c := range testcases {
			r := jsondiff.Diff(c.A, c.B)
			assert.Equal(t, r, c.R, fmt.Sprintf("%d %s", i, c.A))
		}
	})

	t.Run("Diff", func(t *testing.T) {

		testcases := []struct {
			A string
			B string
			R *jsondiff.DiffResult
		}{
			{
				`null`,
				`"null"`,
				&jsondiff.DiffResult{
					Ignores: map[string]jsondiff.DiffItem{},
					Equals:  map[string]jsondiff.DiffItem{},
					Differs: map[string]jsondiff.DiffItem{
						"$": {A: "null", B: `"null"`},
					},
				},
			},
			{
				`3`,
				`"3"`,
				&jsondiff.DiffResult{
					Ignores: map[string]jsondiff.DiffItem{},
					Equals:  map[string]jsondiff.DiffItem{},
					Differs: map[string]jsondiff.DiffItem{
						"$": {A: "3", B: `"3"`},
					},
				},
			},
			{
				`true`,
				`"true"`,
				&jsondiff.DiffResult{
					Ignores: map[string]jsondiff.DiffItem{},
					Equals:  map[string]jsondiff.DiffItem{},
					Differs: map[string]jsondiff.DiffItem{
						"$": {A: "true", B: `"true"`},
					},
				},
			},
			{
				`abc`,
				`"abc"`,
				&jsondiff.DiffResult{
					Ignores: map[string]jsondiff.DiffItem{},
					Equals:  map[string]jsondiff.DiffItem{},
					Differs: map[string]jsondiff.DiffItem{
						"$": {A: "abc", B: `"abc"`},
					},
				},
			},
			{
				`{}`,
				`[]`,
				&jsondiff.DiffResult{
					Ignores: map[string]jsondiff.DiffItem{},
					Equals:  map[string]jsondiff.DiffItem{},
					Differs: map[string]jsondiff.DiffItem{
						`$`: {A: "{}", B: "[]"},
					},
				},
			},
			{
				`{"a":null}`,
				`{"a":3}`,
				&jsondiff.DiffResult{
					Ignores: map[string]jsondiff.DiffItem{},
					Equals:  map[string]jsondiff.DiffItem{},
					Differs: map[string]jsondiff.DiffItem{
						`$[a]`: {A: "null", B: "3"},
					},
				},
			},
			{
				`{"a":3}`,
				`{"a":"abc"}`,
				&jsondiff.DiffResult{
					Ignores: map[string]jsondiff.DiffItem{},
					Equals:  map[string]jsondiff.DiffItem{},
					Differs: map[string]jsondiff.DiffItem{
						`$[a]`: {A: "3", B: `"abc"`},
					},
				},
			},
			{
				`{"a":true}`,
				`{"a":{}}`,
				&jsondiff.DiffResult{
					Ignores: map[string]jsondiff.DiffItem{},
					Equals:  map[string]jsondiff.DiffItem{},
					Differs: map[string]jsondiff.DiffItem{
						`$[a]`: {A: "true", B: "{}"},
					},
				},
			},
			{
				`{"a":{}}`,
				`{"a":[]}`,
				&jsondiff.DiffResult{
					Ignores: map[string]jsondiff.DiffItem{},
					Equals:  map[string]jsondiff.DiffItem{},
					Differs: map[string]jsondiff.DiffItem{
						`$[a]`: {A: "{}", B: "[]"},
					},
				},
			},
			{
				`[3,true,{}]`,
				`["3","abc",[]]`,
				&jsondiff.DiffResult{
					Ignores: map[string]jsondiff.DiffItem{},
					Equals:  map[string]jsondiff.DiffItem{},
					Differs: map[string]jsondiff.DiffItem{
						`$[0]`: {A: "3", B: `"3"`},
						`$[1]`: {A: "true", B: `"abc"`},
						`$[2]`: {A: "{}", B: `[]`},
					},
				},
			},
		}

		for i, c := range testcases {
			r := jsondiff.Diff(c.A, c.B)
			assert.Equal(t, r, c.R, fmt.Sprintf("%d %s", i, c.A))
		}
	})
}
