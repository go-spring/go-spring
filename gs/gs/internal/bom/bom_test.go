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

package bom

import (
	"os"
	"path/filepath"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// writeFakeWorkspace lays down a throwaway repo: a go.work listing the given
// module dirs, plus a versions.yaml baseline, and returns the root path.
func writeFakeWorkspace(t *testing.T, mods map[string]string, baseline string) string {
	t.Helper()
	root := t.TempDir()

	work := "go 1.26\n\nuse (\n"
	for dir := range mods {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "go.mod"), []byte(mods[dir]), 0o644); err != nil {
			t.Fatal(err)
		}
		work += "\t./" + dir + "\n"
	}
	work += ")\n"

	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte(work), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "versions.yaml"), []byte(baseline), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

const fakeBaseline = `
go: "1.26"
dependencies:
  go.opentelemetry.io/otel: v1.43.0
  google.golang.org/grpc: v1.80.0
  github.com/stretchr/testify: v1.11.1
`

func TestLoadBaseline(t *testing.T) {
	root := writeFakeWorkspace(t, map[string]string{}, fakeBaseline)
	base, err := LoadBaseline(filepath.Join(root, "versions.yaml"))
	assert.Error(t, err).Nil()
	assert.That(t, base.Go).Equal("1.26")
	assert.That(t, base.Dependencies["go.opentelemetry.io/otel"]).Equal("v1.43.0")
	assert.That(t, len(base.Dependencies)).Equal(3)
}

func TestFindRoot(t *testing.T) {
	root := writeFakeWorkspace(t, map[string]string{"a": modWith("go-spring.org/a", "")}, fakeBaseline)
	nested := filepath.Join(root, "a")

	got, err := FindRoot(nested)
	assert.Error(t, err).Nil()
	assert.That(t, got).Equal(root)

	_, err = FindRoot(t.TempDir()) // a dir with no go.work above it in TempDir tree
	assert.Error(t, err).NotNil()
}

// modWith renders a minimal go.mod for the given module path with the given
// require block body (already formatted lines, may be empty).
func modWith(path, requires string) string {
	m := "module " + path + "\n\ngo 1.26\n"
	if requires != "" {
		m += "\nrequire (\n" + requires + ")\n"
	}
	return m
}

func TestCheck(t *testing.T) {
	mods := map[string]string{
		// lower than baseline on otel; aligned on grpc.
		"low": modWith("go-spring.org/low",
			"\tgo.opentelemetry.io/otel v1.40.0\n\tgoogle.golang.org/grpc v1.80.0\n"),
		// higher than baseline on otel.
		"high": modWith("go-spring.org/high",
			"\tgo.opentelemetry.io/otel v1.45.0\n"),
		// fully aligned; must produce no drift.
		"ok": modWith("go-spring.org/ok",
			"\tgithub.com/stretchr/testify v1.11.1\n"),
		// depends only on an internal module and an ungoverned dep; no drift.
		"ungoverned": modWith("go-spring.org/ungoverned",
			"\tgo-spring.org/low v0.0.1\n\tgithub.com/some/other v1.2.3\n"),
	}
	root := writeFakeWorkspace(t, mods, fakeBaseline)
	base, err := LoadBaseline(filepath.Join(root, "versions.yaml"))
	assert.Error(t, err).Nil()

	drifts, err := Check(root, base)
	assert.Error(t, err).Nil()
	assert.That(t, len(drifts)).Equal(2)

	byDir := map[string]Drift{}
	for _, d := range drifts {
		byDir[d.Dir] = d
	}
	assert.That(t, byDir["low"].Dep).Equal("go.opentelemetry.io/otel")
	assert.That(t, byDir["low"].Found).Equal("v1.40.0")
	assert.That(t, byDir["low"].Baseline).Equal("v1.43.0")
	assert.That(t, string(byDir["low"].Kind)).Equal("lower")
	assert.That(t, string(byDir["high"].Kind)).Equal("higher")
}

func TestApply(t *testing.T) {
	mods := map[string]string{
		"low": modWith("go-spring.org/low",
			"\tgo.opentelemetry.io/otel v1.40.0\n\tgithub.com/some/other v1.2.3\n"),
		"high": modWith("go-spring.org/high",
			"\tgo.opentelemetry.io/otel v1.45.0\n"),
	}
	root := writeFakeWorkspace(t, mods, fakeBaseline)
	base, err := LoadBaseline(filepath.Join(root, "versions.yaml"))
	assert.Error(t, err).Nil()

	// Align only the "low" module (by workspace dir).
	changes, err := Apply(root, base, "low")
	assert.Error(t, err).Nil()
	assert.That(t, len(changes)).Equal(1)
	assert.That(t, changes[0].Dep).Equal("go.opentelemetry.io/otel")

	// "low" now matches; the ungoverned dep is untouched.
	lowMod, err := os.ReadFile(filepath.Join(root, "low", "go.mod"))
	assert.Error(t, err).Nil()
	assert.String(t, string(lowMod)).Contains("go.opentelemetry.io/otel v1.43.0")
	assert.String(t, string(lowMod)).Contains("github.com/some/other v1.2.3")

	// "high" go.mod must be left exactly as written — Apply targets one module.
	highMod, err := os.ReadFile(filepath.Join(root, "high", "go.mod"))
	assert.Error(t, err).Nil()
	assert.String(t, string(highMod)).Contains("go.opentelemetry.io/otel v1.45.0")

	// Re-running Apply on the aligned module reports no changes.
	changes, err = Apply(root, base, "low")
	assert.Error(t, err).Nil()
	assert.That(t, len(changes)).Equal(0)

	// Unknown target errors out.
	_, err = Apply(root, base, "does-not-exist")
	assert.Error(t, err).NotNil()
}

func TestClassify(t *testing.T) {
	kind, dev := classify("v1.40.0", "v1.43.0")
	assert.That(t, dev).True()
	assert.That(t, string(kind)).Equal("lower")

	kind, dev = classify("v1.45.0", "v1.43.0")
	assert.That(t, dev).True()
	assert.That(t, string(kind)).Equal("higher")

	_, dev = classify("v1.43.0", "v1.43.0")
	assert.That(t, dev).False()
}
