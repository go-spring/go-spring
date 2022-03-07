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

type DiffItem struct {
	A interface{}
	B interface{}
}

type Differ func(a, b string) bool

type diffArg struct {
	ignores map[string]struct{}
	differs map[string]Differ
}

type DiffOption func(arg *diffArg)

func Ignore(a ...string) DiffOption {
	return func(arg *diffArg) {
		for _, k := range a {
			arg.ignores[k] = struct{}{}
		}
	}
}

func Compare(key string, differ Differ) DiffOption {
	return func(arg *diffArg) {
		arg.differs[key] = differ
	}
}

func JsonDiff(a, b []byte, opts ...DiffOption) (map[string]DiffItem, error) {

	arg := diffArg{
		ignores: make(map[string]struct{}),
		differs: make(map[string]Differ),
	}
	for _, opt := range opts {
		opt(&arg)
	}

	ma := Flat(a)
	mb := Flat(b)
	same := make(map[string]struct{})
	result := make(map[string]DiffItem)

	for k, va := range ma {
		_, ok := arg.ignores[k]
		if ok {
			continue
		}
		vb, ok := mb[k]
		if !ok {
			result[k] = DiffItem{A: va}
			continue
		}
		cmp, ok := arg.differs[k]
		if ok && cmp(va, vb) {
			same[k] = struct{}{}
			continue
		}
		if va == vb {
			same[k] = struct{}{}
			continue
		}
		result[k] = DiffItem{A: va, B: vb}
	}

	for k, vb := range mb {
		if _, ok := arg.ignores[k]; ok {
			continue
		}
		if _, ok := same[k]; ok {
			continue
		}
		if _, ok := result[k]; ok {
			continue
		}
		result[k] = DiffItem{B: vb}
	}
	return result, nil
}
