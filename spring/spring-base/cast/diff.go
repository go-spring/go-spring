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

package cast

import (
	"github.com/go-spring/spring-base/json"
)

type DiffItem struct {
	A interface{}
	B interface{}
}

type Differ func(a, b string) bool

type diffArg struct {
	ignores map[string]struct{}
	differs map[string]Differ
	paths   map[string]json.Path
}

func (arg *diffArg) ignored(key string) (bool, error) {
	if _, ok := arg.ignores[key]; ok {
		return true, nil
	}
	for s := range arg.ignores {
		p, err := arg.loadOrStorePath(s)
		if err != nil {
			return false, err
		}
		if p.Matches(key) {
			return true, nil
		}
	}
	return false, nil
}

func (arg *diffArg) getDiffer(key string) (Differ, error) {
	if v, ok := arg.differs[key]; ok {
		return v, nil
	}
	for s, v := range arg.differs {
		p, err := arg.loadOrStorePath(s)
		if err != nil {
			return nil, err
		}
		if p.Matches(key) {
			return v, nil
		}
	}
	return nil, nil
}

func (arg *diffArg) loadOrStorePath(path string) (json.Path, error) {
	if arg.paths == nil {
		arg.paths = make(map[string]json.Path)
	}
	if p, ok := arg.paths[path]; ok {
		return p, nil
	}
	p, err := json.CompilePath(path)
	if err != nil {
		return nil, err
	}
	arg.paths[path] = p
	return p, nil
}

type DiffOption func(arg *diffArg)

// Ignore 忽略某些项。
func Ignore(keys ...string) DiffOption {
	return func(arg *diffArg) {
		for _, key := range keys {
			arg.ignores[key] = struct{}{}
		}
	}
}

// IgnorePath 忽略某些项。
func IgnorePath(paths ...json.Path) DiffOption {
	return func(arg *diffArg) {
		if arg.paths == nil {
			arg.paths = make(map[string]json.Path)
		}
		for _, p := range paths {
			arg.ignores[p.Key()] = struct{}{}
			arg.paths[p.Key()] = p
		}
	}
}

// Compare 设置某些项的比较规则。
func Compare(key string, differ Differ) DiffOption {
	return func(arg *diffArg) {
		arg.differs[key] = differ
	}
}

// ComparePath 设置某些项的比较规则。
func ComparePath(p json.Path, differ Differ) DiffOption {
	return func(arg *diffArg) {
		if arg.paths == nil {
			arg.paths = make(map[string]json.Path)
		}
		arg.differs[p.Key()] = differ
		arg.paths[p.Key()] = p
	}
}

// DiffMap 比较两个映射表。
func DiffMap(a, b map[string]string, opts ...DiffOption) (map[string]DiffItem, error) {

	arg := diffArg{
		ignores: make(map[string]struct{}),
		differs: make(map[string]Differ),
	}
	for _, opt := range opts {
		opt(&arg)
	}

	same := make(map[string]struct{})
	result := make(map[string]DiffItem)

	for k, va := range a {
		var err error
		vb, ok := b[k]
		if !ok {
			if ok, err = arg.ignored(k); err != nil {
				return nil, err
			} else if ok {
				continue
			}
			result[k] = DiffItem{A: va}
			continue
		}
		if va == vb {
			same[k] = struct{}{}
			continue
		}
		if ok, err = arg.ignored(k); err != nil {
			return nil, err
		} else if ok {
			continue
		}
		var cmp Differ
		if cmp, err = arg.getDiffer(k); err != nil {
			return nil, err
		} else if cmp != nil && cmp(va, vb) {
			same[k] = struct{}{}
			continue
		}
		result[k] = DiffItem{A: va, B: vb}
	}

	for k, vb := range b {
		if _, ok := same[k]; ok {
			continue
		}
		if _, ok := result[k]; ok {
			continue
		}
		if ok, err := arg.ignored(k); err != nil {
			return nil, err
		} else if ok {
			continue
		}
		result[k] = DiffItem{B: vb}
	}
	return result, nil
}

// DiffJSON 比较 a,b 执行 FlatJSON 操作之后的结果。
func DiffJSON(a, b interface{}, opts ...DiffOption) (map[string]DiffItem, error) {
	return DiffMap(FlatJSON(a), FlatJSON(b), opts...)
}
