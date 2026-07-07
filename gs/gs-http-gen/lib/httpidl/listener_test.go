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

package httpidl

import (
	"bytes"
	"os"
	"testing"

	"go-spring.org/stdlib/jsonflow"
	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/stdlib/testing/require"
)

func TestListener(t *testing.T) {
	fileName := "testdata/success/http.idl"
	b, err := os.ReadFile(fileName)
	require.Error(t, err).Nil()
	doc, _, err := ParseIDL(b)
	require.Error(t, err).Nil()
	b, err = os.ReadFile("testdata/success/http.formated.idl")
	require.Error(t, err).Nil()
	s := Format(doc)
	assert.String(t, s).Equal(string(b))
	b, err = os.ReadFile("testdata/success/http.idl.json")
	require.Error(t, err).Nil()
	v, err := jsonflow.MarshalIndent(doc, "", "  ")
	require.Error(t, err).Nil()
	b = bytes.TrimSpace(b)
	assert.String(t, string(v)).Equal(string(b))
}

func TestParseIDLRejectsDuplicateTopLevelName(t *testing.T) {
	_, _, err := ParseIDL([]byte(`
type User {
    required string name
}

type User {
    required string id
}
`))
	require.Error(t, err).NotNil()
	assert.String(t, err.Error()).Contains("duplicate type name User")
}

func TestParseIDLRejectsDuplicateRPCName(t *testing.T) {
	_, _, err := ParseIDL([]byte(`
type Req {
    required string id
}

type Resp {
    required string ok
}

rpc Save(Req) Resp {
    method="POST"
    path="/save"
    contentType="json"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}

rpc Save(Req) Resp {
    method="PUT"
    path="/save"
    contentType="json"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}
`))
	require.Error(t, err).NotNil()
	assert.String(t, err.Error()).Contains("duplicate rpc name Save")
}
