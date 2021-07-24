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

package assert_test

import (
	"errors"
	"testing"

	"github.com/go-spring/spring-core/util/assert"
)

func TestCheck(t *testing.T) {

	var r struct {
		True  bool
		False bool
		Nil   interface{}
	}

	err := assert.Check(assert.Cases{
		{r.True, "r.True want true but is false"},
		{!r.False, "r.False want false but is true"},
		{r.Nil == nil, "r.Nil want nil but not nil"},
	})

	assert.Error(t, err, "r.True want true but is false")
}

func checkFailed(t *testing.T) {
	if t.Failed() {
		t.Fatalf("failed but expect not failed")
	}
}

func TestTrue(t *testing.T) {
	assert.True(t, true)
	checkFailed(t)
}

func TestFalse(t *testing.T) {
	assert.False(t, false)
	checkFailed(t)
}

func TestNil(t *testing.T) {
	assert.Nil(t, nil)
	checkFailed(t)
}

func TestNotNil(t *testing.T) {
	assert.NotNil(t, new(int))
	checkFailed(t)
}

func TestEqual(t *testing.T) {
	assert.Equal(t, 0, 0)
	checkFailed(t)
}

func TestNotEqual(t *testing.T) {
	assert.NotEqual(t, 1, 0)
	checkFailed(t)
}

func TestPanic(t *testing.T) {
	assert.Panic(t, func() { panic("error") }, "error")
	checkFailed(t)
}

func TestMatches(t *testing.T) {
	assert.Matches(t, "this is an error", "this is an error")
	checkFailed(t)
}

func TestError(t *testing.T) {
	assert.Error(t, errors.New("this is an error"), "an error")
	checkFailed(t)
}
