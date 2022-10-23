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

package util_test

import (
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/code"
	"github.com/go-spring/spring-base/util"
)

func TestError(t *testing.T) {

	e0 := util.Error(code.FileLine(), "error: 0")
	t.Log(e0)
	assert.Error(t, e0, ".*/error_test.go:29 error: 0")

	e1 := util.Errorf(code.FileLine(), "error: %d", 1)
	t.Log(e1)
	assert.Error(t, e1, ".*/error_test.go:33 error: 1")

	e2 := util.Wrap(e0, code.FileLine(), "error: 0")
	t.Log(e2)
	assert.Error(t, e2, ".*/error_test.go:37 error: 0; .*/error_test.go:29 error: 0")

	e3 := util.Wrapf(e1, code.FileLine(), "error: %d", 1)
	t.Log(e3)
	assert.Error(t, e3, ".*/error_test.go:41 error: 1; .*/error_test.go:33 error: 1")

	e4 := util.Wrap(e2, code.FileLine(), "error: 0")
	t.Log(e4)
	assert.Error(t, e4, ".*/error_test.go:45 error: 0; .*/error_test.go:37 error: 0; .*/error_test.go:29 error: 0")

	e5 := util.Wrapf(e3, code.FileLine(), "error: %d", 1)
	t.Log(e5)
	assert.Error(t, e5, ".*/error_test.go:49 error: 1; .*/error_test.go:41 error: 1; .*/error_test.go:33 error: 1")
}
