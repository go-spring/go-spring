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
	"strings"
)

// MatchResult 路径匹配结果。
type MatchResult int

const (
	MatchNone   = MatchResult(0) // 匹配失败
	MatchPrefix = MatchResult(1) // 前缀匹配
	MatchFull   = MatchResult(2) // 全部匹配
)

// JsonPath 使用 JSON 路径表达式定义的路径。
type JsonPath interface {
	Match(path string) MatchResult
}

type jsonPath struct {
	expr string
}

// ToJsonPath 解析使用 JSON 路径表达式定义的路径。
func ToJsonPath(expr string) JsonPath {
	return &jsonPath{expr: expr}
}

// Match 匹配使用 JSON 路径表达式定义的路径，返回匹配结果。
func (p *jsonPath) Match(path string) MatchResult {
	if p.expr == path {
		return MatchFull
	}
	if strings.HasPrefix(p.expr, path) {
		return MatchPrefix
	}
	return MatchNone
}
