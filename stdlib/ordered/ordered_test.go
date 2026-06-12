/*
 * Copyright 2025 The Go-Spring Authors.
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

package ordered

import (
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
)

func TestOrderedMapKeys(t *testing.T) {
	strMap := map[string]int{
		"banana": 2,
		"apple":  1,
		"cherry": 3,
	}
	actualStrKeys := MapKeys(strMap)
	expectedStrKeys := []string{"apple", "banana", "cherry"}
	assert.Slice(t, actualStrKeys).Equal(expectedStrKeys)

	intMap := map[int]string{
		3: "three",
		1: "one",
		2: "two",
	}
	actualIntKeys := MapKeys(intMap)
	expectedIntKeys := []int{1, 2, 3}
	assert.Slice(t, actualIntKeys).Equal(expectedIntKeys)

	emptyMap := map[string]int{}
	actualEmptyKeys := MapKeys(emptyMap)
	var expectedEmptyKeys []string
	assert.Slice(t, actualEmptyKeys).Equal(expectedEmptyKeys)

	singleMap := map[string]int{"only": 1}
	actualSingleKeys := MapKeys(singleMap)
	expectedSingleKeys := []string{"only"}
	assert.Slice(t, actualSingleKeys).Equal(expectedSingleKeys)
}
