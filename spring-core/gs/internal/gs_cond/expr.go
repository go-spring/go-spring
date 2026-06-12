/*
 * Copyright 2024 The Go-Spring Authors.
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

package gs_cond

import (
	"maps"

	"github.com/expr-lang/expr"
	"github.com/go-spring/stdlib/errutil"
)

var funcMap = map[string]any{}

// RegisterExpressFunc registers a function under the given name, making it available
// for use in expressions evaluated by EvalExpr.
// Must be called in init functions only.
func RegisterExpressFunc(name string, fn any) {
	funcMap[name] = fn
}

// EvalExpr evaluates a boolean expression using the provided value as the "$" variable.
// - `input` is a boolean expression string to evaluate, it must return a boolean result.
// - `val` is a string value accessible as "$" within the expression context.
func EvalExpr(input string, val string) (bool, error) {
	env := map[string]any{"$": val}
	maps.Copy(env, funcMap)
	r, err := expr.Eval(input, env)
	if err != nil {
		return false, err
	}
	ret, ok := r.(bool)
	if !ok {
		return false, errutil.Explain(nil, "expression must return a boolean value")
	}
	return ret, nil
}
