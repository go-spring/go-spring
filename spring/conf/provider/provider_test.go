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
	"context"
	"errors"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// testProvider is a hand-rolled Provider whose Close records the call and can
// be made to fail, so CloseAll's contract is observable.
type testProvider struct {
	content map[string]string
	closes  *int
	closeFn func() error
}

func (p testProvider) Load(optional bool, source string) (map[string]string, error) {
	return p.content, nil
}

func (p testProvider) Close(context.Context) error {
	if p.closes != nil {
		*p.closes++
	}
	if p.closeFn != nil {
		return p.closeFn()
	}
	return nil
}

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
	Register(name, ProviderFunc(func(optional bool, source string) (map[string]string, error) {
		gotOptional = optional
		gotSource = source
		return map[string]string{"loaded": "true"}, nil
	}))

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

func TestCloseAllClosesEveryProvider(t *testing.T) {
	const name1 = "closableProvider1ForTest"
	const name2 = "closableProvider2ForTest"
	defer delete(providers, name1)
	defer delete(providers, name2)

	var closes int
	Register(name1, testProvider{closes: &closes})
	Register(name2, testProvider{closes: &closes})

	assert.That(t, CloseAll(context.Background())).Nil()
	assert.That(t, closes).Equal(2)
}

func TestCloseAllReportsFailureAndContinues(t *testing.T) {
	const failing = "failingProviderForTest"
	const healthy = "healthyProviderForTest"
	defer delete(providers, failing)
	defer delete(providers, healthy)

	var closes int
	Register(failing, testProvider{
		closes:  &closes,
		closeFn: func() error { return errors.New("boom") },
	})
	Register(healthy, testProvider{closes: &closes})

	err := CloseAll(context.Background())
	assert.Error(t, err).Matches("close provider failingProviderForTest error")
	// A failing provider must not skip the rest.
	assert.That(t, closes).Equal(2)
}

func TestCloseAllKeepsRegistryForTheNextInstance(t *testing.T) {
	// Close runs once per application instance, while registration happens in
	// init: a process running two instances in sequence (gs.RunTest) must still
	// find its providers after the first Close.
	const name = "survivesCloseProviderForTest"
	defer delete(providers, name)

	Register(name, testProvider{content: map[string]string{"loaded": "true"}})

	assert.That(t, CloseAll(context.Background())).Nil()

	m, err := Load(name + ":anything")
	assert.That(t, err).Nil()
	assert.That(t, m).Equal(map[string]string{"loaded": "true"})
}

func TestProviderFuncCloseIsNoop(t *testing.T) {
	assert.That(t, ProviderFunc(func(bool, string) (map[string]string, error) {
		return nil, nil
	}).Close(context.Background())).Nil()
}

func TestLoadUnsupportedProvider(t *testing.T) {
	_, err := Load("unknown-provider:./config.yaml")
	assert.Error(t, err).Matches("unsupported provider type unknown-provider")
}

func TestLoadFile(t *testing.T) {

	t.Run("optional missing file returns nil", func(t *testing.T) {
		m, err := Load("optional:file:./missing.yaml")
		assert.That(t, err).Nil()
		assert.That(t, m).Nil()
	})

	t.Run("required missing file returns error", func(t *testing.T) {
		_, err := Load("file:./missing.yaml")
		assert.Error(t, err).Matches("no such file or directory")
	})

	t.Run("existing file", func(t *testing.T) {
		m, err := Load("../testdata/config/app.properties")
		assert.That(t, err).Nil()
		assert.That(t, m).Equal(map[string]string{
			"properties.list[0]":          "1",
			"properties.list[1]":          "2",
			"properties.obj.list[0].age":  "4",
			"properties.obj.list[0].name": "tom",
			"properties.obj.list[1].age":  "2",
			"properties.obj.list[1].name": "jerry",
		})
	})
}
