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

var empty = &emptyValidator{}

func init() {
	validate.Register("empty", empty)
}

type emptyValidator struct {
	count int
}

func (d *emptyValidator) reset() {
	d.count = 0
}

func (d *emptyValidator) Field(tag string, i interface{}) error {
	d.count++
	return nil
}

func TestField(t *testing.T) {
	i := 6

	err := validate.Field("empty:\"\"", i)
	assert.Nil(t, err)
	assert.Equal(t, empty.count, 1)

	err = validate.Field("expr:\"$>=3\"", i)
	assert.Nil(t, err)

	err = validate.Field("expr:\"$<3\"", i)
	assert.Error(t, err, "validate failed on \"\\$<3\" for value 6")

	err = validate.Field("expr:\"$<3\"", "abc")
	assert.Error(t, err, "invalid operation\\: string \\< int \\(1:2\\)")
}
