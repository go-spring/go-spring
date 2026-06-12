/*
 * Copyright 2024 The Go-Spring Authors.
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

package gs_conf

import (
	"os"
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
)

func TestExtractCmdArgs(t *testing.T) {
	t.Run("no args - empty", func(t *testing.T) {
		os.Args = nil

		p, err := extractCmdArgs()
		assert.That(t, err).Nil()
		assert.That(t, p).NotNil()
	})

	t.Run("no args - only executable", func(t *testing.T) {
		os.Args = []string{"test"}

		p, err := extractCmdArgs()
		assert.That(t, err).Nil()
		assert.That(t, p).NotNil()
	})

	t.Run("normal", func(t *testing.T) {
		os.Args = []string{"test", "-D", "name=go-spring", "-D", "debug"}

		p, err := extractCmdArgs()
		assert.That(t, err).Nil()
		v, ok := p.Get("name")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("go-spring")
		v, ok = p.Get("debug")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("true")
	})

	t.Run("missing arg", func(t *testing.T) {
		os.Args = []string{"test", "-D"}

		p, err := extractCmdArgs()
		assert.Error(t, err).Matches("cmd option -D requires an argument")
		assert.That(t, p).Nil()
	})

	t.Run("custom prefix", func(t *testing.T) {
		os.Args = []string{"test", "--option", "port=8080"}

		oldEnv := os.Getenv(CommandArgsPrefix)
		defer func() { _ = os.Setenv(CommandArgsPrefix, oldEnv) }()
		_ = os.Setenv(CommandArgsPrefix, "--option")

		p, err := extractCmdArgs()
		assert.That(t, err).Nil()
		v, ok := p.Get("port")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("8080")
	})

	t.Run("ignore args", func(t *testing.T) {
		os.Args = []string{"test", "-v", "-D", "env=prod", "--log-level=info"}

		p, err := extractCmdArgs()
		assert.That(t, err).Nil()
		v, ok := p.Get("env")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("prod")
		_, ok = p.Get("--log-level")
		assert.That(t, ok).False()
		_, ok = p.Get("-v")
		assert.That(t, ok).False()
	})

	t.Run("empty value assignment", func(t *testing.T) {
		os.Args = []string{"test", "-D", "name="}

		p, err := extractCmdArgs()
		assert.That(t, err).Nil()
		v, ok := p.Get("name")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("")
	})

	t.Run("empty key after trim", func(t *testing.T) {
		// When key is empty after trimming, it should return an error
		os.Args = []string{"test", "-D", "   =value"}

		p, err := extractCmdArgs()
		assert.Error(t, err).Matches("cmd option -D has empty key")
		assert.That(t, p).Nil()
	})

	t.Run("custom prefix with spaces", func(t *testing.T) {
		os.Args = []string{"test", "-D", "database.url=localhost:3306"}

		p, err := extractCmdArgs()
		assert.That(t, err).Nil()
		v, ok := p.Get("database.url")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("localhost:3306")
	})

	t.Run("mixed args", func(t *testing.T) {
		os.Args = []string{"test", "-D", "valid=key", "-x", "-D", "another=value"}

		p, err := extractCmdArgs()
		assert.That(t, err).Nil()
		v, ok := p.Get("valid")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("key")
		v, ok = p.Get("another")
		assert.That(t, ok).True()
		assert.That(t, v).Equal("value")
		_, ok = p.Get("-x")
		assert.That(t, ok).False()
	})
}
