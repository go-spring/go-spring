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

package feature

import (
	"os"
	"path/filepath"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/stdlib/testing/require"
)

// grpcManifest is a minimal manifest carrying one server feature that owns a
// dir, a file, and both an internal and third-party init import.
func grpcManifest() *Manifest {
	return &Manifest{Features: []Feature{{
		Key: "grpc",
		Owns: Owns{
			Dirs:  []string{"idl/grpc", "internal/api/server/grpcsvr"},
			Files: []string{"internal/api/controller/order/order_controller_grpc.go"},
			InitImports: []string{
				"GS_PROJECT_MODULE/internal/api/server/grpcsvr",
				"go-spring.org/starter-grpc",
			},
		},
	}}}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.Error(t, os.MkdirAll(filepath.Dir(path), 0o755)).Nil()
	require.Error(t, os.WriteFile(path, []byte(content), 0o644)).Nil()
}

func TestCopy(t *testing.T) {
	replaces := map[string]string{"GS_PROJECT_MODULE": "github.com/you/demo"}

	newLayout := func(t *testing.T) string {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "idl/grpc/pb/svc.go"), "package pb // GS_PROJECT_MODULE")
		writeFile(t, filepath.Join(dir, "internal/api/server/grpcsvr/grpcsvr.go"), "package grpcsvr\nimport _ \"GS_PROJECT_MODULE/idl/grpc/pb\"")
		writeFile(t, filepath.Join(dir, "internal/api/controller/order/order_controller_grpc.go"), "package order // grpc")
		return dir
	}

	newProject := func(t *testing.T) string {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "internal/init.go"), "package domain\n\nimport (\n\t_ \"github.com/you/demo/internal/api/job\"\n\n\t_ \"go-spring.org/starter-gorm-mysql\"\n)\n")
		return dir
	}

	t.Run("copies dirs, files, replaces placeholders, inserts imports", func(t *testing.T) {
		layout, project := newLayout(t), newProject(t)
		err := Copy(project, layout, grpcManifest(), map[string]struct{}{"grpc": {}}, replaces)
		assert.That(t, err).Nil()

		// Dir + file copied with placeholder resolved.
		b, err := os.ReadFile(filepath.Join(project, "internal/api/server/grpcsvr/grpcsvr.go"))
		assert.That(t, err).Nil()
		assert.String(t, string(b)).Equal("package grpcsvr\nimport _ \"github.com/you/demo/idl/grpc/pb\"")
		_, err = os.Stat(filepath.Join(project, "internal/api/controller/order/order_controller_grpc.go"))
		assert.That(t, err).Nil()

		// init.go: server import in the internal group, starter in third-party.
		got, err := os.ReadFile(filepath.Join(project, "internal/init.go"))
		assert.That(t, err).Nil()
		assert.String(t, string(got)).Equal("package domain\n\nimport (\n"+
			"\t_ \"github.com/you/demo/internal/api/job\"\n"+
			"\t_ \"github.com/you/demo/internal/api/server/grpcsvr\"\n\n"+
			"\t_ \"go-spring.org/starter-gorm-mysql\"\n"+
			"\t_ \"go-spring.org/starter-grpc\"\n"+
			")\n")
	})

	t.Run("already added dir is refused", func(t *testing.T) {
		layout, project := newLayout(t), newProject(t)
		require.Error(t, os.MkdirAll(filepath.Join(project, "internal/api/server/grpcsvr"), 0o755)).Nil()
		err := Copy(project, layout, grpcManifest(), map[string]struct{}{"grpc": {}}, replaces)
		assert.Error(t, err).Matches("already added")
	})

	t.Run("unselected feature copies nothing", func(t *testing.T) {
		layout, project := newLayout(t), newProject(t)
		err := Copy(project, layout, grpcManifest(), map[string]struct{}{}, replaces)
		assert.That(t, err).Nil()
		_, err = os.Stat(filepath.Join(project, "internal/api/server/grpcsvr"))
		assert.That(t, os.IsNotExist(err)).True()
	})
}
