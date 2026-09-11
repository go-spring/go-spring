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

package contract

import (
	"encoding/json"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestDecode_SingleAndArray(t *testing.T) {
	cs, err := decode([]byte("\n\t {\"name\":\"a\",\"request\":{\"method\":\"GET\",\"path\":\"/x\"},\"response\":{\"status\":200}} \n"))
	assert.Error(t, err).Nil()
	assert.That(t, len(cs)).Equal(1)
	assert.String(t, cs[0].Name).Equal("a")

	cs, err = decode([]byte("  [{\"name\":\"a\",\"request\":{\"method\":\"GET\",\"path\":\"/x\"},\"response\":{\"status\":200}},{\"name\":\"b\",\"request\":{\"method\":\"GET\",\"path\":\"/y\"},\"response\":{\"status\":200}}]"))
	assert.Error(t, err).Nil()
	assert.That(t, len(cs)).Equal(2)
	assert.String(t, cs[1].Name).Equal("b")
}

func TestDecode_EmptyIsNoContracts(t *testing.T) {
	cs, err := decode(nil)
	assert.Error(t, err).Nil()
	assert.That(t, len(cs)).Equal(0)

	cs, err = decode([]byte(" \t\r\n "))
	assert.Error(t, err).Nil()
	assert.That(t, len(cs)).Equal(0)
}

func TestDecode_MalformedJSONFails(t *testing.T) {
	_, err := decode([]byte("{not json"))
	assert.Error(t, err).NotNil()
}

func TestDecode_MissingRequiredFieldsFail(t *testing.T) {
	_, err := decode([]byte(`{"name":"a","request":{"path":"/x"},"response":{}}`))
	assert.Error(t, err).NotNil()
	assert.String(t, err.Error()).Matches(".*request\\.method is required.*")

	_, err = decode([]byte(`{"name":"a","request":{"method":"GET"},"response":{}}`))
	assert.Error(t, err).NotNil()
	assert.String(t, err.Error()).Matches(".*request\\.path is required.*")

	_, err = decode([]byte(`{"name":"a","request":{"method":"GET","path":"/x"},"response":{"status":42}}`))
	assert.Error(t, err).NotNil()
	assert.String(t, err.Error()).Matches(".*not a valid HTTP status code.*")

	// The array layout validates every element.
	_, err = decode([]byte(`[{"name":"a","request":{"method":"GET","path":"/x"},"response":{"status":200}},{"request":{"path":"/x"}}]`))
	assert.Error(t, err).NotNil()
	assert.String(t, err.Error()).Matches(".*request\\.method is required.*")

	// Status omitted is rejected too.
	_, err = decode([]byte(`{"name":"a","request":{"method":"GET","path":"/x"},"response":{}}`))
	assert.Error(t, err).NotNil()
	assert.String(t, err.Error()).Matches(".*response\\.status.*missing or not a valid.*")
}

func TestValuesUnmarshal_SingleOrArray(t *testing.T) {
	var r Request
	err := json.Unmarshal([]byte(`{"method":"GET","path":"/x","query":{"name":"Ada","tag":["a","b"]},"headers":{"Accept":["text/plain","application/json"]}}`), &r)
	assert.Error(t, err).Nil()
	assert.That(t, len(r.Query["name"])).Equal(1)
	assert.String(t, r.Query.Get("name")).Equal("Ada")
	assert.That(t, len(r.Query["tag"])).Equal(2)
	assert.That(t, len(r.Headers["Accept"])).Equal(2)

	// Anything but a string or an array of strings is rejected.
	err = json.Unmarshal([]byte(`{"method":"GET","path":"/x","query":{"tag":42}}`), &r)
	assert.Error(t, err).NotNil()
	assert.String(t, err.Error()).Matches(".*tag.*")
}
