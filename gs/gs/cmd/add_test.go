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

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/stdlib/testing/require"
)

func TestReadProjectMeta(t *testing.T) {
	t.Run("valid gs.json", func(t *testing.T) {
		dir := t.TempDir()
		require.Error(t, os.WriteFile(filepath.Join(dir, "gs.json"),
			[]byte(`{"module":"github.com/you/demo","lang":"en","layout_version":"v1.2.3"}`), 0o644)).Nil()

		meta, err := readProjectMeta(dir)
		assert.That(t, err).Nil()
		assert.String(t, meta.Module).Equal("github.com/you/demo")
		assert.String(t, meta.Lang).Equal("en")
		assert.String(t, meta.LayoutVersion).Equal("v1.2.3")
	})

	t.Run("missing gs.json is not a project root", func(t *testing.T) {
		_, err := readProjectMeta(t.TempDir())
		assert.Error(t, err).Matches("not found")
	})

	t.Run("missing layout_version rejected", func(t *testing.T) {
		dir := t.TempDir()
		require.Error(t, os.WriteFile(filepath.Join(dir, "gs.json"),
			[]byte(`{"module":"github.com/you/demo"}`), 0o644)).Nil()
		_, err := readProjectMeta(dir)
		assert.Error(t, err).Matches("layout_version")
	})

	t.Run("lang defaults to zh", func(t *testing.T) {
		dir := t.TempDir()
		require.Error(t, os.WriteFile(filepath.Join(dir, "gs.json"),
			[]byte(`{"module":"github.com/you/demo","layout_version":"v1.0.0"}`), 0o644)).Nil()
		meta, err := readProjectMeta(dir)
		assert.That(t, err).Nil()
		assert.String(t, meta.Lang).Equal("zh")
	})
}

func TestModuleLeaf(t *testing.T) {
	assert.String(t, moduleLeaf("github.com/you/demo")).Equal("demo")
	assert.String(t, moduleLeaf("demo")).Equal("demo")
}
