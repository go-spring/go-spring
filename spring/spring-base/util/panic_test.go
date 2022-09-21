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
	"errors"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/util"
)

func TestPanicCond(t *testing.T) {

	util.Panic("this is an error").When(false)
	assert.Panic(t, func() {
		util.Panic("this is an error").When(true)
	}, "this is an error")

	util.Panic(errors.New("this is an error")).When(false)
	assert.Panic(t, func() {
		util.Panic(errors.New("this is an error")).When(true)
	}, "this is an error")

	util.Panicf("this is an %s", "error").When(false)
	assert.Panic(t, func() {
		util.Panicf("this is an %s", "error").When(true)
	}, "this is an error")
}
