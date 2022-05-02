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

	e0 := util.Error(code.FileLine(), "error")
	assert.Error(t, e0, ".*/error_test.go:29 error")

	e1 := util.Errorf(code.FileLine(), "error: %d", 0)
	assert.Error(t, e1, ".*/error_test.go:32 error: 0")

	e2 := util.Wrap(e0, code.FileLine(), "error")
	assert.Error(t, e2, ".*/error_test.go:35 error\n.*/error_test.go:29 error")

	e3 := util.Wrapf(e1, code.FileLine(), "error: %d", 1)
	assert.Error(t, e3, ".*/error_test.go:38 error: 1\n.*/error_test.go:32 error: 0")
}
