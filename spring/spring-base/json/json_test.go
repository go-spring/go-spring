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
	"bytes"
	stdJson "encoding/json"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/json"
)

func TestJSON(t *testing.T) {

	var ss []string
	err := json.Unmarshal([]byte(`{}`), &ss)
	assert.Error(t, err, "json: cannot unmarshal object into Go value of type \\[\\]string")

	_, err = json.Marshal(complex(1.0, 0.5))
	assert.Error(t, err, "json: unsupported type: complex128")

	_, err = json.MarshalIndent(complex(1.0, 0.5), "", "  ")
	assert.Error(t, err, "json: unsupported type: complex128")

	m := map[string]string{"a": "3"}
	b, err := json.Marshal(m)
	assert.Nil(t, err)
	assert.Equal(t, string(b), `{"a":"3"}`)

	b, err = json.MarshalIndent(m, "", "  ")
	assert.Nil(t, err)
	assert.Equal(t, string(b), "{\n  \"a\": \"3\"\n}")

	m = map[string]string{}
	err = json.Unmarshal([]byte(`{"b":"5"}`), &m)
	assert.Nil(t, err)
	assert.Equal(t, m, map[string]string{"b": "5"})

	w := bytes.NewBuffer(nil)
	e := json.NewEncoder(w)
	{
		e1 := e.(*json.WrapEncoder).E.(*stdJson.Encoder)
		e1.SetIndent("", "  ")
	}
	err = e.Encode(m)
	assert.Nil(t, err)
	assert.Equal(t, w.String(), "{\n  \"b\": \"5\"\n}\n")

	err = e.Encode(complex(1.0, 0.5))
	assert.Error(t, err, "json: unsupported type: complex128")

	r := bytes.NewReader([]byte("{\n  \"b\": \"3\"\n}\n"))
	d := json.NewDecoder(r)
	{
		d1 := d.(*json.WrapDecoder).D.(*stdJson.Decoder)
		d1.UseNumber()
	}
	err = d.Decode(&m)
	assert.Nil(t, err)
	assert.Equal(t, m, map[string]string{"b": "3"})

	r.Reset([]byte(`{"a":"3"}`))
	err = d.Decode(&ss)
	assert.Error(t, err, "json: cannot unmarshal object into Go value of type \\[\\]string")
}
