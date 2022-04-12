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

package differ_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/differ"
)

func TestDiffJSON(t *testing.T) {

	t.Run("Equal & UnquoteExpand", func(t *testing.T) {

		testcases := []struct {
			A string
			B string
			S func(*differ.JsonDiffer)
			R *differ.JsonDiffResult
		}{
			{
				`null`,
				`null`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						"$": {A: "null", B: "null"},
					},
				},
			},
			{
				`3`,
				`3`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						"$": {A: "3", B: "3"},
					},
				},
			},
			{
				`"3"`,
				`"3"`,
				func(d *differ.JsonDiffer) {
					d.Path(differ.ToJsonPath("$")).UnquoteExpand()
				},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[""]`: {A: "3", B: "3"},
					},
				},
			},
			{
				`true`,
				`true`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						"$": {A: "true", B: "true"},
					},
				},
			},
			{
				`"true"`,
				`"true"`,
				func(d *differ.JsonDiffer) {
					d.Path(differ.ToJsonPath("$")).UnquoteExpand()
				},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[""]`: {A: "true", B: "true"},
					},
				},
			},
			{
				`abc`,
				`abc`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						"$": {A: "abc", B: "abc"},
					},
				},
			},
			{
				`"abc"`,
				`"abc"`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$`: {A: `"abc"`, B: `"abc"`},
					},
				},
			},
			{
				`{}`,
				`{}`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$`: {A: "{}", B: "{}"},
					},
				},
			},
			{
				`"{}"`,
				`"{}"`,
				func(d *differ.JsonDiffer) {
					d.Path(differ.ToJsonPath("$")).UnquoteExpand()
				},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[""]`: {A: "{}", B: "{}"},
					},
				},
			},
			{
				`[]`,
				`[]`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$`: {A: "[]", B: "[]"},
					},
				},
			},
			{
				`"[]"`,
				`"[]"`,
				func(d *differ.JsonDiffer) {
					d.Path(differ.ToJsonPath("$")).UnquoteExpand()
				},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[""]`: {A: "[]", B: "[]"},
					},
				},
			},
			{
				`{"a":null}`,
				`{"a":null}`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: "null", B: "null"},
					},
				},
			},
			{
				`{"a":3}`,
				`{"a":3}`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: "3", B: "3"},
					},
				},
			},
			{
				`{"a":"3"}`,
				`{"a":"3"}`,
				func(d *differ.JsonDiffer) {
					d.Path(differ.ToJsonPath("$[a]")).UnquoteExpand()
				},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a][""]`: {A: "3", B: "3"},
					},
				},
			},
			{
				`{"a":true}`,
				`{"a":true}`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: "true", B: "true"},
					},
				},
			},
			{
				`{"a":"true"}`,
				`{"a":"true"}`,
				func(d *differ.JsonDiffer) {
					d.Path(differ.ToJsonPath("$[a]")).UnquoteExpand()
				},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a][""]`: {A: "true", B: "true"},
					},
				},
			},
			{
				`{"a":"b"}`,
				`{"a":"b"}`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: `"b"`, B: `"b"`},
					},
				},
			},
			{
				`{"a":{}}`,
				`{"a":{}}`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: "{}", B: "{}"},
					},
				},
			},
			{
				`{"a":"{}"}`,
				`{"a":"{}"}`,
				func(d *differ.JsonDiffer) {
					d.Path(differ.ToJsonPath("$[a]")).UnquoteExpand()
				},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a][""]`: {A: "{}", B: "{}"},
					},
				},
			},
			{
				`{"a":[]}`,
				`{"a":[]}`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: "[]", B: "[]"},
					},
				},
			},
			{
				`{"a":"[]"}`,
				`{"a":"[]"}`,
				func(d *differ.JsonDiffer) {
					d.Path(differ.ToJsonPath("$[a]")).UnquoteExpand()
				},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a][""]`: {A: "[]", B: "[]"},
					},
				},
			},
			{
				`[3,"3"]`,
				`[3,"3"]`,
				func(d *differ.JsonDiffer) {
					d.Path(differ.ToJsonPath("$[1]")).UnquoteExpand()
				},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[0]`:     {A: "3", B: "3"},
						`$[1][""]`: {A: "3", B: "3"},
					},
				},
			},
			{
				`[true,"true"]`,
				`[true,"true"]`,
				func(d *differ.JsonDiffer) {
					d.Path(differ.ToJsonPath("$[1]")).UnquoteExpand()
				},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[0]`:     {A: "true", B: "true"},
						`$[1][""]`: {A: "true", B: "true"},
					},
				},
			},
			{
				`[null,"null"]`,
				`[null,"null"]`,
				func(d *differ.JsonDiffer) {
					d.Path(differ.ToJsonPath("$[1]")).UnquoteExpand()
				},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[0]`:     {A: "null", B: "null"},
						`$[1][""]`: {A: "null", B: "null"},
					},
				},
			},
			{
				`[a]`,
				`[a]`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$`: {A: "[a]", B: "[a]"},
					},
				},
			},
			{
				`["a"]`,
				`["a"]`,
				func(d *differ.JsonDiffer) {},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[0]`: {A: `"a"`, B: `"a"`},
					},
				},
			},
			{
				`[{},"{}"]`,
				`[{},"{}"]`,
				func(d *differ.JsonDiffer) {
					d.Path(differ.ToJsonPath("$[1]")).UnquoteExpand()
				},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[0]`:     {A: "{}", B: "{}"},
						`$[1][""]`: {A: "{}", B: "{}"},
					},
				},
			},
			{
				`[[],"[]"]`,
				`[[],"[]"]`,
				func(d *differ.JsonDiffer) {
					d.Path(differ.ToJsonPath("$[1]")).UnquoteExpand()
				},
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[0]`:     {A: "[]", B: "[]"},
						`$[1][""]`: {A: "[]", B: "[]"},
					},
				},
			},
		}

		for i, c := range testcases {
			d := differ.NewJsonDiffer()
			c.S(d)
			r := d.Diff(c.A, c.B)
			assert.Equal(t, r, c.R, fmt.Sprintf("%d %s", i, c.A))
		}
	})

	t.Run("Equal & No UnquoteExpand", func(t *testing.T) {

		testcases := []struct {
			A string
			B string
			R *differ.JsonDiffResult
		}{
			{
				`null`,
				`null`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						"$": {A: "null", B: "null"},
					},
				},
			},
			{
				`3`,
				`3`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						"$": {A: "3", B: "3"},
					},
				},
			},
			{
				`"3"`,
				`"3"`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$`: {A: `"3"`, B: `"3"`},
					},
				},
			},
			{
				`true`,
				`true`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						"$": {A: "true", B: "true"},
					},
				},
			},
			{
				`"true"`,
				`"true"`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$`: {A: `"true"`, B: `"true"`},
					},
				},
			},
			{
				`abc`,
				`abc`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						"$": {A: "abc", B: "abc"},
					},
				},
			},
			{
				`"abc"`,
				`"abc"`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$`: {A: `"abc"`, B: `"abc"`},
					},
				},
			},
			{
				`{}`,
				`{}`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$`: {A: "{}", B: "{}"},
					},
				},
			},
			{
				`"{}"`,
				`"{}"`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$`: {A: `"{}"`, B: `"{}"`},
					},
				},
			},
			{
				`[]`,
				`[]`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$`: {A: "[]", B: "[]"},
					},
				},
			},
			{
				`"[]"`,
				`"[]"`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$`: {A: `"[]"`, B: `"[]"`},
					},
				},
			},
			{
				`{"a":null}`,
				`{"a":null}`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: "null", B: "null"},
					},
				},
			},
			{
				`{"a":3}`,
				`{"a":3}`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: "3", B: "3"},
					},
				},
			},
			{
				`{"a":"3"}`,
				`{"a":"3"}`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: `"3"`, B: `"3"`},
					},
				},
			},
			{
				`{"a":true}`,
				`{"a":true}`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: "true", B: "true"},
					},
				},
			},
			{
				`{"a":"true"}`,
				`{"a":"true"}`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: `"true"`, B: `"true"`},
					},
				},
			},
			{
				`{"a":"b"}`,
				`{"a":"b"}`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: `"b"`, B: `"b"`},
					},
				},
			},
			{
				`{"a":{}}`,
				`{"a":{}}`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: "{}", B: "{}"},
					},
				},
			},
			{
				`{"a":"{}"}`,
				`{"a":"{}"}`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: `"{}"`, B: `"{}"`},
					},
				},
			},
			{
				`{"a":[]}`,
				`{"a":[]}`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: "[]", B: "[]"},
					},
				},
			},
			{
				`{"a":"[]"}`,
				`{"a":"[]"}`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[a]`: {A: `"[]"`, B: `"[]"`},
					},
				},
			},
			{
				`[3,"3"]`,
				`[3,"3"]`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[0]`: {A: "3", B: "3"},
						`$[1]`: {A: `"3"`, B: `"3"`},
					},
				},
			},
			{
				`[true,"true"]`,
				`[true,"true"]`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[0]`: {A: "true", B: "true"},
						`$[1]`: {A: `"true"`, B: `"true"`},
					},
				},
			},
			{
				`[null,"null"]`,
				`[null,"null"]`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[0]`: {A: "null", B: "null"},
						`$[1]`: {A: `"null"`, B: `"null"`},
					},
				},
			},
			{
				`[a]`,
				`[a]`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$`: {A: "[a]", B: "[a]"},
					},
				},
			},
			{
				`["a"]`,
				`["a"]`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[0]`: {A: `"a"`, B: `"a"`},
					},
				},
			},
			{
				`[{},"{}"]`,
				`[{},"{}"]`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[0]`: {A: "{}", B: "{}"},
						`$[1]`: {A: `"{}"`, B: `"{}"`},
					},
				},
			},
			{
				`[[],"[]"]`,
				`[[],"[]"]`,
				&differ.JsonDiffResult{
					Differs: map[string]differ.JsonDiffItem{},
					Ignores: map[string]differ.JsonDiffItem{},
					Equals: map[string]differ.JsonDiffItem{
						`$[0]`: {A: "[]", B: "[]"},
						`$[1]`: {A: `"[]"`, B: `"[]"`},
					},
				},
			},
		}

		for i, c := range testcases {
			r := differ.DiffJSON(c.A, c.B)
			assert.Equal(t, r, c.R, fmt.Sprintf("%d %s", i, c.A))
		}
	})

	t.Run("Diff", func(t *testing.T) {

		testcases := []struct {
			A string
			B string
			R *differ.JsonDiffResult
		}{
			{
				`null`,
				`"null"`,
				&differ.JsonDiffResult{
					Ignores: map[string]differ.JsonDiffItem{},
					Equals:  map[string]differ.JsonDiffItem{},
					Differs: map[string]differ.JsonDiffItem{
						"$": {A: "null", B: `"null"`},
					},
				},
			},
			{
				`3`,
				`"3"`,
				&differ.JsonDiffResult{
					Ignores: map[string]differ.JsonDiffItem{},
					Equals:  map[string]differ.JsonDiffItem{},
					Differs: map[string]differ.JsonDiffItem{
						"$": {A: "3", B: `"3"`},
					},
				},
			},
			{
				`true`,
				`"true"`,
				&differ.JsonDiffResult{
					Ignores: map[string]differ.JsonDiffItem{},
					Equals:  map[string]differ.JsonDiffItem{},
					Differs: map[string]differ.JsonDiffItem{
						"$": {A: "true", B: `"true"`},
					},
				},
			},
			{
				`abc`,
				`"abc"`,
				&differ.JsonDiffResult{
					Ignores: map[string]differ.JsonDiffItem{},
					Equals:  map[string]differ.JsonDiffItem{},
					Differs: map[string]differ.JsonDiffItem{
						"$": {A: "abc", B: `"abc"`},
					},
				},
			},
			{
				`{}`,
				`[]`,
				&differ.JsonDiffResult{
					Ignores: map[string]differ.JsonDiffItem{},
					Equals:  map[string]differ.JsonDiffItem{},
					Differs: map[string]differ.JsonDiffItem{
						`$`: {A: "{}", B: "[]"},
					},
				},
			},
			{
				`{"a":null}`,
				`{"a":3}`,
				&differ.JsonDiffResult{
					Ignores: map[string]differ.JsonDiffItem{},
					Equals:  map[string]differ.JsonDiffItem{},
					Differs: map[string]differ.JsonDiffItem{
						`$[a]`: {A: "null", B: "3"},
					},
				},
			},
			{
				`{"a":3}`,
				`{"a":"abc"}`,
				&differ.JsonDiffResult{
					Ignores: map[string]differ.JsonDiffItem{},
					Equals:  map[string]differ.JsonDiffItem{},
					Differs: map[string]differ.JsonDiffItem{
						`$[a]`: {A: "3", B: `"abc"`},
					},
				},
			},
			{
				`{"a":true}`,
				`{"a":{}}`,
				&differ.JsonDiffResult{
					Ignores: map[string]differ.JsonDiffItem{},
					Equals:  map[string]differ.JsonDiffItem{},
					Differs: map[string]differ.JsonDiffItem{
						`$[a]`: {A: "true", B: "{}"},
					},
				},
			},
			{
				`{"a":{}}`,
				`{"a":[]}`,
				&differ.JsonDiffResult{
					Ignores: map[string]differ.JsonDiffItem{},
					Equals:  map[string]differ.JsonDiffItem{},
					Differs: map[string]differ.JsonDiffItem{
						`$[a]`: {A: "{}", B: "[]"},
					},
				},
			},
			{
				`[3,true,{}]`,
				`["3","abc",[]]`,
				&differ.JsonDiffResult{
					Ignores: map[string]differ.JsonDiffItem{},
					Equals:  map[string]differ.JsonDiffItem{},
					Differs: map[string]differ.JsonDiffItem{
						`$[0]`: {A: "3", B: `"3"`},
						`$[1]`: {A: "true", B: `"abc"`},
						`$[2]`: {A: "{}", B: `[]`},
					},
				},
			},
		}

		for i, c := range testcases {
			r := differ.DiffJSON(c.A, c.B)
			assert.Equal(t, r, c.R, fmt.Sprintf("%d %s", i, c.A))
		}
	})
}
