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

// Package jsondiff ...
package jsondiff

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
)

// Strategy 比较策略。
type Strategy int

const (
	IgnorePath        = Strategy(1 << 0) // 忽略匹配路径
	IgnoreValue       = Strategy(1 << 1) // 忽略路径的值
	IgnoreArrayOrder  = Strategy(1 << 2) // 忽略元素的顺序
	IgnoreExtraItems  = Strategy(1 << 3) // 忽略多余的元素
	TreatNullAsAbsent = Strategy(1 << 4) // 将 null 视为字段缺失
	UnquoteExpand     = Strategy(1 << 5) // 解析引号里面的内容
)

// MatchResult 路径匹配结果。
type MatchResult int

const (
	MatchNone   = MatchResult(0) // 匹配失败
	MatchPrefix = MatchResult(1) // 前缀匹配
	MatchFull   = MatchResult(2) // 全部匹配
)

// Comparator 值比较器。
type Comparator func(a, b interface{}) bool

type Config struct {
	path       string
	strategy   Strategy
	comparator Comparator
}

func Path(path string) *Config {
	return &Config{path: path}
}

func (c *Config) isIgnorePath() bool {
	return c.strategy&IgnorePath == IgnorePath
}

func (c *Config) IgnorePath() *Config {
	c.strategy |= IgnorePath
	return c
}

func (c *Config) isIgnoreValue() bool {
	return c.strategy&IgnoreValue == IgnoreValue
}

func (c *Config) IgnoreValue() *Config {
	c.strategy |= IgnoreValue
	return c
}

func (c *Config) isIgnoreArrayOrder() bool {
	return c.strategy&IgnoreArrayOrder == IgnoreArrayOrder
}

func (c *Config) IgnoreArrayOrder() *Config {
	c.strategy |= IgnoreArrayOrder
	return c
}

func (c *Config) isIgnoreExtraItems() bool {
	return c.strategy&IgnoreExtraItems == IgnoreExtraItems
}

func (c *Config) IgnoreExtraItems() *Config {
	c.strategy |= IgnoreExtraItems
	return c
}

func (c *Config) isTreatNullAsAbsent() bool {
	return c.strategy&TreatNullAsAbsent == TreatNullAsAbsent
}

func (c *Config) TreatNullAsAbsent() *Config {
	c.strategy |= TreatNullAsAbsent
	return c
}

func (c *Config) isUnquoteExpand() bool {
	return c.strategy&UnquoteExpand == UnquoteExpand
}

func (c *Config) UnquoteExpand() *Config {
	c.strategy |= UnquoteExpand
	return c
}

func (c *Config) SetComparator(comparator Comparator) *Config {
	c.comparator = comparator
	return c
}

func (c *Config) Match(path string) MatchResult {
	if c.path == path {
		return MatchFull
	}
	if strings.HasPrefix(c.path, path) {
		return MatchPrefix
	}
	return MatchNone
}

type DiffItem struct {
	A string
	B string
}

type DiffResult struct {
	Differs map[string]DiffItem
	Ignores map[string]DiffItem
	Equals  map[string]DiffItem
}

// difference JSON 比较器。
type difference struct {
	configs []*Config
}

func decode(data []byte) (interface{}, error) {
	var v interface{}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// Diff 比较 a,b 两个 JSON 字符串，返回它们异同之处。
func (d *difference) Diff(a, b string) *DiffResult {
	prefix := "$"
	result := &DiffResult{
		Differs: make(map[string]DiffItem),
		Ignores: make(map[string]DiffItem),
		Equals:  make(map[string]DiffItem),
	}
	va, errA := decode([]byte(a))
	vb, errB := decode([]byte(b))
	if errA != nil || errB != nil {
		if a != b {
			result.Differs[prefix] = DiffItem{
				A: toString(UnquoteString(a)),
				B: toString(UnquoteString(b)),
			}
		} else {
			result.Equals[prefix] = DiffItem{
				A: toString(UnquoteString(a)),
				B: toString(UnquoteString(b)),
			}
		}
	} else {
		param := &diffParam{configs: d.configs}
		diffValue(prefix, va, vb, param, result)
	}
	return result
}

type diffParam struct {
	Config
	configs []*Config
}

func diffValue(prefix string, a, b interface{}, parent *diffParam, result *DiffResult) {

	if a == nil && b == nil {
		result.Equals[prefix] = DiffItem{
			A: toString(a),
			B: toString(b),
		}
		return
	}

	current := &diffParam{}

	for _, c := range parent.configs {
		r := c.Match(prefix)
		if r == MatchFull {
			current.Config = *c
			break
		} else if r == MatchPrefix {
			current.configs = append(current.configs, c)
		}
	}

	if current.isIgnorePath() {
		result.Ignores[prefix] = DiffItem{
			A: toString(a),
			B: toString(b),
		}
		return
	}

	if current.isIgnoreValue() {
		if reflect.TypeOf(a) == reflect.TypeOf(b) {
			result.Ignores[prefix] = DiffItem{
				A: toString(a),
				B: toString(b),
			}
		} else {
			result.Differs[prefix] = DiffItem{
				A: toString(a),
				B: toString(b),
			}
		}
		return
	}

	if current.isUnquoteExpand() {
		sa, okA := a.(string)
		sb, okB := b.(string)
		if !okA || !okB {
			result.Differs[prefix] = DiffItem{
				A: toString(a),
				B: toString(b),
			}
			return
		}
		va, errA := decode([]byte(sa))
		vb, errB := decode([]byte(sb))
		if errA != nil || errB != nil {
			result.Differs[prefix] = DiffItem{
				A: toString(a),
				B: toString(b),
			}
			return
		}
		diffValue(prefix+`[""]`, va, vb, current, result)
		return
	}

	if current.comparator != nil {
		if current.comparator(a, b) {
			result.Equals[prefix] = DiffItem{
				A: toString(a),
				B: toString(b),
			}
		} else {
			result.Differs[prefix] = DiffItem{
				A: toString(a),
				B: toString(b),
			}
		}
		return
	}

	switch va := a.(type) {
	case map[string]interface{}:
		if vb, ok := b.(map[string]interface{}); ok {
			diffMap(prefix, va, vb, current, result)
			return
		}
	case []interface{}:
		if vb, ok := b.([]interface{}); ok {
			if current.isIgnoreArrayOrder() {
				diffSliceIgnoreOrder(prefix, va, vb, current, result)
			} else {
				diffSliceHaveOrder(prefix, va, vb, current, result)
			}
			return
		}
	case AbsentValue:
		if _, ok := b.(AbsentValue); ok {
			result.Equals[prefix] = DiffItem{
				A: toString(a),
				B: toString(b),
			}
			return
		}
	case json.Number:
		if vb, ok := b.(json.Number); ok {
			diffStringOrBool(prefix, va, vb, current, result)
			return
		}
	case string:
		if vb, ok := b.(string); ok {
			diffStringOrBool(prefix, va, vb, current, result)
			return
		}
	case bool:
		if vb, ok := b.(bool); ok {
			diffStringOrBool(prefix, va, vb, current, result)
			return
		}
	}

	result.Differs[prefix] = DiffItem{
		A: toString(a),
		B: toString(b),
	}
}

func diffMap(prefix string, a, b map[string]interface{}, param *diffParam, result *DiffResult) {

	if len(a) == 0 && len(b) == 0 {
		result.Equals[prefix] = DiffItem{
			A: toString(a),
			B: toString(b),
		}
		return
	}

	visit := map[string]struct{}{}
	for key, va := range a {
		vb, ok := b[key]
		if param.isTreatNullAsAbsent() {
			if va == nil {
				va = AbsentValue("null")
			}
			if !ok {
				vb = AbsentValue("")
			} else if vb == nil {
				vb = AbsentValue("null")
			}
		}
		visit[key] = struct{}{}
		key = prefix + "[" + key + "]"
		diffValue(key, va, vb, param, result)
	}

	for key, vb := range b {
		if _, ok := visit[key]; ok {
			continue
		}
		va := AbsentValue("")
		key = prefix + "[" + key + "]"
		if param.isIgnoreExtraItems() {
			result.Ignores[key] = DiffItem{
				A: toString(va),
				B: toString(vb),
			}
		} else {
			diffValue(key, va, vb, param, result)
		}
	}
}

func diffSliceHaveOrder(prefix string, a, b []interface{}, param *diffParam, result *DiffResult) {

	if len(a) == 0 && len(b) == 0 {
		result.Equals[prefix] = DiffItem{
			A: toString(a),
			B: toString(b),
		}
		return
	}

	for i := 0; i < len(a); i++ {
		key := prefix + "[" + strconv.Itoa(i) + "]"
		if i < len(b) {
			diffValue(key, a[i], b[i], param, result)
		} else {
			diffValue(key, a[i], AbsentValue(""), param, result)
		}
	}

	for i := len(a); i < len(b); i++ {
		vb := b[i]
		va := AbsentValue("")
		key := prefix + "[" + strconv.Itoa(i) + "]"
		if param.isIgnoreExtraItems() {
			result.Ignores[key] = DiffItem{
				A: toString(va),
				B: toString(vb),
			}
		} else {
			diffValue(key, va, vb, param, result)
		}
	}
}

func diffSliceIgnoreOrder(prefix string, a, b []interface{}, param *diffParam, result *DiffResult) {

}

func diffStringOrBool(prefix string, va, vb interface{}, param *diffParam, result *DiffResult) {
	if va != vb {
		result.Differs[prefix] = DiffItem{
			A: toString(va),
			B: toString(vb),
		}
	} else {
		result.Equals[prefix] = DiffItem{
			A: toString(va),
			B: toString(vb),
		}
	}
}

type (
	AbsentValue   string
	UnquoteString string
)

func toString(v interface{}) string {
	switch m := v.(type) {
	case map[string]interface{}:
		if len(m) == 1 {
			var key string
			for k := range m {
				key = k
			}
			if key == `""` {
				return strconv.Quote(toString(m[key]))
			}
		}
	case []interface{}:
		for i := range m {
			m[i] = toString(m[i])
		}
	case UnquoteString:
		return string(m)
	case AbsentValue:
		return string(m)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// Diff 比较 a,b 两个 JSON 字符串，返回它们异同之处。
func Diff(a, b string, configs ...*Config) *DiffResult {
	d := &difference{configs: configs}
	return d.Diff(a, b)
}
