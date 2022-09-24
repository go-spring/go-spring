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

package json_test

import (
	stdJson "encoding/json"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-replay/internal/json"
)

func TestString(t *testing.T) {
	var src, dst struct {
		S string `json:"s"`
	}
	src.S = "\u0000\xC0\n\t\u0000\xBEm\u0006\x89Z(\u0000\n"
	b1, err := stdJson.Marshal(&src)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := json.Marshal(&src)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, string(b1), `{"s":"\u0000\ufffd\n\t\u0000\ufffdm\u0006\ufffdZ(\u0000\n"}`)
	assert.Equal(t, string(b2), `{"s":"@\"\\x00\\xc0\\n\\t\\x00\\xbem\\x06\\x89Z(\\x00\\n\""}`)
	err = json.Unmarshal(b2, &dst)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, dst.S, "\u0000\xC0\n\t\u0000\xBEm\u0006\x89Z(\u0000\n")
}
