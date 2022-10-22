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
	"encoding/xml"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/web/binding"
)

type XMLBindParamCommon struct {
	A string   `xml:"a"`
	B []string `xml:"b"`
}

type XMLBindParam struct {
	XMLBindParamCommon
	C int   `xml:"c"`
	D []int `xml:"d"`
}

func TestBindXML(t *testing.T) {

	data, err := xml.Marshal(&XMLBindParam{
		XMLBindParamCommon: XMLBindParamCommon{
			A: "1",
			B: []string{"2", "3"},
		},
		C: 4,
		D: []int{5, 6},
	})
	assert.Nil(t, err)

	r := &MockRequest{
		contentType: binding.MIMEApplicationXML,
		requestBody: string(data),
	}

	expect := XMLBindParam{
		XMLBindParamCommon: XMLBindParamCommon{
			A: "1",
			B: []string{"2", "3"},
		},
		C: 4,
		D: []int{5, 6},
	}

	var p XMLBindParam
	err = binding.Bind(&p, r)
	assert.Nil(t, err)
	assert.Equal(t, p, expect)
}
