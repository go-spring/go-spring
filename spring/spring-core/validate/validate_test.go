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

package validate_test

import (
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/validate"
)

func TestTagName(t *testing.T) {
	assert.Equal(t, validate.TagName(), "expr")
}

func TestStruct(t *testing.T) {
	var s struct {
		E string `expr:"len($)>3"`
	}
	err := validate.Struct(&s)
	assert.Nil(t, err)
}

func TestField(t *testing.T) {
	i := 2
	err := validate.Field(i, "")
	assert.Nil(t, err)
	err = validate.Field(i, "len($)==5")
	assert.Error(t, err, "returns error")
	err = validate.Field(i, "$>=3")
	assert.Error(t, err, "validate failed on \"\\$>=3\" for value 2")
	i = 5
	err = validate.Field(i, "$>=3")
	assert.Nil(t, err)
}
