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

package differ

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"

	"github.com/go-spring/spring-base/differ/path"
)

const (
	jsonIgnorePath        = Strategy(1 << 0) // 忽略匹配路径
	jsonIgnoreValue       = Strategy(1 << 1) // 忽略路径的值
	jsonIgnoreArrayOrder  = Strategy(1 << 2) // 忽略元素的顺序
	jsonIgnoreExtraItems  = Strategy(1 << 3) // 忽略多余的元素
	jsonTreatNullAsAbsent = Strategy(1 << 4) // 将 null 视为字段缺失
	jsonUnquoteExpand     = Strategy(1 << 5) // 解析引号里面的内容
)

type jsonPathConfig struct {
	strategy   Strategy
	comparator Comparator
}

func (c *jsonPathConfig) ignorePath() bool {
	return c.strategy&jsonIgnorePath == jsonIgnorePath
}

func (c *jsonPathConfig) IgnorePath() *jsonPathConfig {
	c.strategy |= jsonIgnorePath
	return c
}

func (c *jsonPathConfig) ignoreValue() bool {
	return c.strategy&jsonIgnoreValue == jsonIgnoreValue
}

func (c *jsonPathConfig) IgnoreValue() *jsonPathConfig {
	c.strategy |= jsonIgnoreValue
	return c
}

func (c *jsonPathConfig) ignoreArrayOrder() bool {
	return c.strategy&jsonIgnoreArrayOrder == jsonIgnoreArrayOrder
}

func (c *jsonPathConfig) IgnoreArrayOrder() *jsonPathConfig {
	c.strategy |= jsonIgnoreArrayOrder
	return c
}

func (c *jsonPathConfig) ignoreExtraItems() bool {
	return c.strategy&jsonIgnoreExtraItems == jsonIgnoreExtraItems
}

func (c *jsonPathConfig) IgnoreExtraItems() *jsonPathConfig {
	c.strategy |= jsonIgnoreExtraItems
	return c
}

func (c *jsonPathConfig) treatNullAsAbsent() bool {
	return c.strategy&jsonTreatNullAsAbsent == jsonTreatNullAsAbsent
}

func (c *jsonPathConfig) TreatNullAsAbsent() *jsonPathConfig {
	c.strategy |= jsonTreatNullAsAbsent
	return c
}

func (c *jsonPathConfig) unquoteExpand() bool {
	return c.strategy&jsonUnquoteExpand == jsonUnquoteExpand
}

func (c *jsonPathConfig) UnquoteExpand() *jsonPathConfig {
	c.strategy |= jsonUnquoteExpand
	return c
}

func (c *jsonPathConfig) SetComparator(comparator Comparator) *jsonPathConfig {
	c.comparator = comparator
	return c
}

type JsonDiffItem struct {
	A string
	B string
}

type JsonDiffResult struct {
	Differs map[string]JsonDiffItem
	Ignores map[string]JsonDiffItem
	Equals  map[string]JsonDiffItem
}

// JsonDiffer JSON 比较器。
type JsonDiffer struct {
	config map[path.JsonPath]*jsonPathConfig
}

// NewJsonDiffer 创建新的 JSON 比较器。
func NewJsonDiffer() *JsonDiffer {
	return &JsonDiffer{
		config: make(map[path.JsonPath]*jsonPathConfig),
	}
}

// Path 获取路径的配置。
func (d *JsonDiffer) Path(expr string) *jsonPathConfig {
	p := path.ParseJsonPath(expr)
	c, ok := d.config[p]
	if !ok {
		c = &jsonPathConfig{}
		d.config[p] = c
	}
	return c
}

func decodeJson(data []byte) (interface{}, error) {
	var v interface{}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// Diff 比较 a,b 两个 JSON 字符串，返回它们异同之处。
func (d *JsonDiffer) Diff(a, b string) *JsonDiffResult {
	prefix := "$"
	result := &JsonDiffResult{
		Differs: make(map[string]JsonDiffItem),
		Ignores: make(map[string]JsonDiffItem),
		Equals:  make(map[string]JsonDiffItem),
	}
	va, errA := decodeJson([]byte(a))
	vb, errB := decodeJson([]byte(b))
	if errA != nil || errB != nil {
		if a != b {
			result.Differs[prefix] = JsonDiffItem{
				A: toJsonString(UnquoteString(a)),
				B: toJsonString(UnquoteString(b)),
			}
		} else {
			result.Equals[prefix] = JsonDiffItem{
				A: toJsonString(UnquoteString(a)),
				B: toJsonString(UnquoteString(b)),
			}
		}
	} else {
		param := diffParam{
			config: d.config,
		}
		diffValue(prefix, va, vb, param, result)
	}
	return result
}

type diffParam struct {
	jsonPathConfig
	config map[path.JsonPath]*jsonPathConfig
}

func diffValue(prefix string, a, b interface{}, parent diffParam, result *JsonDiffResult) {

	if a == nil && b == nil {
		result.Equals[prefix] = JsonDiffItem{
			A: toJsonString(a),
			B: toJsonString(b),
		}
		return
	}

	current := diffParam{
		config: make(map[path.JsonPath]*jsonPathConfig),
	}

	for p, c := range parent.config {
		r := p.Match(prefix)
		if r == path.MatchFull {
			current.jsonPathConfig = *c
			break
		} else if r == path.MatchPrefix {
			current.config[p] = c
		}
	}

	if current.ignorePath() {
		result.Ignores[prefix] = JsonDiffItem{
			A: toJsonString(a),
			B: toJsonString(b),
		}
		return
	}

	if current.ignoreValue() {
		if reflect.TypeOf(a) == reflect.TypeOf(b) {
			result.Ignores[prefix] = JsonDiffItem{
				A: toJsonString(a),
				B: toJsonString(b),
			}
		} else {
			result.Differs[prefix] = JsonDiffItem{
				A: toJsonString(a),
				B: toJsonString(b),
			}
		}
		return
	}

	if current.unquoteExpand() {
		sa, okA := a.(string)
		sb, okB := b.(string)
		if !okA || !okB {
			result.Differs[prefix] = JsonDiffItem{
				A: toJsonString(a),
				B: toJsonString(b),
			}
			return
		}
		va, errA := decodeJson([]byte(sa))
		vb, errB := decodeJson([]byte(sb))
		if errA != nil || errB != nil {
			result.Differs[prefix] = JsonDiffItem{
				A: toJsonString(a),
				B: toJsonString(b),
			}
			return
		}
		diffValue(prefix+`[""]`, va, vb, current, result)
		return
	}

	if current.comparator != nil {
		if current.comparator(a, b) {
			result.Equals[prefix] = JsonDiffItem{
				A: toJsonString(a),
				B: toJsonString(b),
			}
		} else {
			result.Differs[prefix] = JsonDiffItem{
				A: toJsonString(a),
				B: toJsonString(b),
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
			if current.ignoreArrayOrder() {
				diffSliceIgnoreOrder(prefix, va, vb, current, result)
			} else {
				diffSliceHaveOrder(prefix, va, vb, current, result)
			}
			return
		}
	case AbsentValue:
		if _, ok := b.(AbsentValue); ok {
			result.Equals[prefix] = JsonDiffItem{
				A: toJsonString(a),
				B: toJsonString(b),
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

	result.Differs[prefix] = JsonDiffItem{
		A: toJsonString(a),
		B: toJsonString(b),
	}
}

func diffMap(prefix string, a, b map[string]interface{}, param diffParam, result *JsonDiffResult) {

	if len(a) == 0 && len(b) == 0 {
		result.Equals[prefix] = JsonDiffItem{
			A: toJsonString(a),
			B: toJsonString(b),
		}
		return
	}

	visit := map[string]struct{}{}
	for key, va := range a {
		vb, ok := b[key]
		if param.treatNullAsAbsent() {
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
		if param.ignoreExtraItems() {
			result.Ignores[key] = JsonDiffItem{
				A: toJsonString(va),
				B: toJsonString(vb),
			}
		} else {
			diffValue(key, va, vb, param, result)
		}
	}
}

func diffSliceHaveOrder(prefix string, a, b []interface{}, param diffParam, result *JsonDiffResult) {

	if len(a) == 0 && len(b) == 0 {
		result.Equals[prefix] = JsonDiffItem{
			A: toJsonString(a),
			B: toJsonString(b),
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
		if param.ignoreExtraItems() {
			result.Ignores[key] = JsonDiffItem{
				A: toJsonString(va),
				B: toJsonString(vb),
			}
		} else {
			diffValue(key, va, vb, param, result)
		}
	}
}

func diffSliceIgnoreOrder(prefix string, a, b []interface{}, param diffParam, result *JsonDiffResult) {

}

func diffStringOrBool(prefix string, va, vb interface{}, param diffParam, result *JsonDiffResult) {
	if va != vb {
		result.Differs[prefix] = JsonDiffItem{
			A: toJsonString(va),
			B: toJsonString(vb),
		}
	} else {
		result.Equals[prefix] = JsonDiffItem{
			A: toJsonString(va),
			B: toJsonString(vb),
		}
	}
}

type (
	AbsentValue   string
	UnquoteString string
)

func toJsonString(v interface{}) string {
	switch m := v.(type) {
	case map[string]interface{}:
		if len(m) == 1 {
			var key string
			for k := range m {
				key = k
			}
			if key == `""` {
				return strconv.Quote(toJsonString(m[key]))
			}
		}
	case []interface{}:
		for i := range m {
			m[i] = toJsonString(m[i])
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

// DiffJSON 比较 a,b 两个 JSON 字符串，返回它们异同之处。
func DiffJSON(a, b string) *JsonDiffResult {
	return NewJsonDiffer().Diff(a, b)
}
