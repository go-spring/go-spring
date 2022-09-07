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

package gsl

import (
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/go-spring/spring-base/code"
	"github.com/go-spring/spring-base/util"
)

// Eval returns the value for the expression expr.
func Eval(expr string, val string) (bool, error) {

	if _, err := strconv.ParseInt(val, 10, 64); err == nil {
		expr = strings.ReplaceAll(expr, "$", val)
	} else if _, err = strconv.ParseUint(val, 10, 64); err == nil {
		expr = strings.ReplaceAll(expr, "$", val)
	} else if _, err = strconv.ParseFloat(val, 64); err == nil {
		expr = strings.ReplaceAll(expr, "$", val)
	} else if _, err = strconv.ParseBool(val); err == nil {
		expr = strings.ReplaceAll(expr, "$", val)
	} else {
		expr = strings.ReplaceAll(expr, "$", strconv.Quote(val))
	}

	r, err := types.Eval(token.NewFileSet(), nil, token.NoPos, expr)
	if err != nil {
		return false, util.Wrapf(err, code.FileLine(), "eval %q returns error", expr)
	}
	b, err := strconv.ParseBool(r.Value.String())
	if err != nil {
		return false, util.Wrapf(err, code.FileLine(), "eval %q returns error", expr)
	}
	return b, nil
}
