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

package cast_test

import (
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cast"
)

func TestIsHexDigit(t *testing.T) {

	assert.False(t, cast.IsHexDigit('/'))
	assert.True(t, cast.IsHexDigit('0'))
	assert.True(t, cast.IsHexDigit('9'))
	assert.False(t, cast.IsHexDigit(':'))

	assert.False(t, cast.IsHexDigit('`'))
	assert.True(t, cast.IsHexDigit('a'))
	assert.True(t, cast.IsHexDigit('f'))
	assert.False(t, cast.IsHexDigit('g'))

	assert.False(t, cast.IsHexDigit('@'))
	assert.True(t, cast.IsHexDigit('A'))
	assert.True(t, cast.IsHexDigit('F'))
	assert.False(t, cast.IsHexDigit('G'))
}

func TestHexDigitToInt(t *testing.T) {

	assert.Equal(t, cast.HexDigitToInt('/'), -1)
	assert.Equal(t, cast.HexDigitToInt('0'), 0)
	assert.Equal(t, cast.HexDigitToInt('9'), 9)
	assert.Equal(t, cast.HexDigitToInt(':'), -1)

	assert.Equal(t, cast.HexDigitToInt('`'), -1)
	assert.Equal(t, cast.HexDigitToInt('a'), 10)
	assert.Equal(t, cast.HexDigitToInt('f'), 15)
	assert.Equal(t, cast.HexDigitToInt('g'), -1)

	assert.Equal(t, cast.HexDigitToInt('@'), -1)
	assert.Equal(t, cast.HexDigitToInt('A'), 10)
	assert.Equal(t, cast.HexDigitToInt('F'), 15)
	assert.Equal(t, cast.HexDigitToInt('G'), -1)
}
