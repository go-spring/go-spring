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

package binding_test

import (
	"encoding/json"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/web/binding"
)

type JSONBindParamCommon struct {
	A string   `json:"a"`
	B []string `json:"b"`
}

type JSONBindParam struct {
	JSONBindParamCommon
	C int   `json:"c"`
	D []int `json:"d"`
}

func TestBindJSON(t *testing.T) {

	data, err := json.Marshal(map[string]interface{}{
		"a": "1",
		"b": []string{"2", "3"},
		"c": 4,
		"d": []int64{5, 6},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := &MockRequest{
		contentType: binding.MIMEApplicationJSON,
		requestBody: string(data),
	}

	expect := JSONBindParam{
		JSONBindParamCommon: JSONBindParamCommon{
			A: "1",
			B: []string{"2", "3"},
		},
		C: 4,
		D: []int{5, 6},
	}

	var p JSONBindParam
	err = binding.Bind(&p, ctx)
	assert.Nil(t, err)
	assert.Equal(t, p, expect)
}
