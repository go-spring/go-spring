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

package provider

import (
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
)

func TestRegisterNilProvider(t *testing.T) {
	const name = "nilProviderForTest"
	var p Provider
	defer delete(providers, name)

	assert.Panic(t, func() {
		Register(name, p)
	}, "provider nilProviderForTest cannot be nil")
}

func TestLoadCustomProviderSourceWithColon(t *testing.T) {
	const name = "colonProviderForTest"
	defer delete(providers, name)

	var gotOptional bool
	var gotSource string
	Register(name, func(optional bool, source string) (map[string]string, error) {
		gotOptional = optional
		gotSource = source
		return map[string]string{"loaded": "true"}, nil
	})

	m, err := Load(name + ":localhost:2379/config")
	assert.That(t, err).Nil()
	assert.That(t, m).Equal(map[string]string{"loaded": "true"})
	assert.That(t, gotOptional).False()
	assert.That(t, gotSource).Equal("localhost:2379/config")

	m, err = Load("optional:" + name + ":localhost:2379/config")
	assert.That(t, err).Nil()
	assert.That(t, m).Equal(map[string]string{"loaded": "true"})
	assert.That(t, gotOptional).True()
	assert.That(t, gotSource).Equal("localhost:2379/config")
}
