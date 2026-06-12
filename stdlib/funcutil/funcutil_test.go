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

package funcutil_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/stdlib/funcutil"
	"github.com/go-spring/stdlib/testing/assert"
)

func TestFuncName(t *testing.T) {
	assert.That(t, funcutil.FuncName(func() {})).Equal("funcutil_test.TestFuncName.func1")
	assert.That(t, funcutil.FuncName(func(i int) {})).Equal("funcutil_test.TestFuncName.func2")
	assert.That(t, funcutil.FuncName(fnNoArgs)).Equal("funcutil_test.fnNoArgs")
	assert.That(t, funcutil.FuncName(fnWithArgs)).Equal("funcutil_test.fnWithArgs")
	assert.That(t, funcutil.FuncName((*receiver).ptrFnNoArgs)).Equal("funcutil_test.(*receiver).ptrFnNoArgs")
	assert.That(t, funcutil.FuncName((*receiver).ptrFnWithArgs)).Equal("funcutil_test.(*receiver).ptrFnWithArgs")
}

// nolint: unused
func fnNoArgs() {}

// nolint: unused
func fnWithArgs(i int) {}

type receiver struct{}

// nolint: unused
func (r *receiver) ptrFnNoArgs() {}

// nolint: unused
func (r *receiver) ptrFnWithArgs(i int) {}

func TestFileLine(t *testing.T) {
	testcases := []struct {
		fn     any
		file   string
		line   int
		fnName string
	}{
		{
			fnNoArgs,
			"funcutil/funcutil_test.go",
			37,
			"funcutil_test.fnNoArgs",
		},
		{
			fnWithArgs,
			"funcutil/funcutil_test.go",
			40,
			"funcutil_test.fnWithArgs",
		},
		{
			(*receiver).ptrFnNoArgs,
			"funcutil/funcutil_test.go",
			45,
			"funcutil_test.(*receiver).ptrFnNoArgs",
		},
		{
			(*receiver).ptrFnWithArgs,
			"funcutil/funcutil_test.go",
			48,
			"funcutil_test.(*receiver).ptrFnWithArgs",
		},
	}
	for i, c := range testcases {
		file, line, fnName := funcutil.FileLine(c.fn)
		assert.That(t, line).Equal(c.line, fmt.Sprint(i))
		assert.That(t, fnName).Equal(c.fnName, fmt.Sprint(i))
		assert.String(t, file).HasSuffix(c.file, fmt.Sprint(i))
	}
}
