package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/stdlib/testing/require"
)

func TestToPascal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single word",
			input:    "hello",
			expected: "Hello",
		},
		{
			name:     "snake case with two words",
			input:    "hello_world",
			expected: "HelloWorld",
		},
		{
			name:     "snake case with multiple words",
			input:    "hello_world_test_case",
			expected: "HelloWorldTestCase",
		},
		{
			name:     "leading underscore",
			input:    "_hello_world",
			expected: "HelloWorld",
		},
		{
			name:     "trailing underscore",
			input:    "hello_world_",
			expected: "HelloWorld",
		},
		{
			name:     "multiple consecutive underscores",
			input:    "hello__world",
			expected: "HelloWorld",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only underscores",
			input:    "___",
			expected: "",
		},
		{
			name:     "already pascal case",
			input:    "HelloWorld",
			expected: "HelloWorld",
		},
		{
			name:     "with numbers",
			input:    "hello_2_world",
			expected: "Hello2World",
		},
		{
			name:     "mixed case input",
			input:    "Hello_world_Test",
			expected: "HelloWorldTest",
		},
		{
			name:     "kebab case",
			input:    "hello-world",
			expected: "HelloWorld",
		},
		{
			name:     "mixed kebab and snake",
			input:    "hello-world_test",
			expected: "HelloWorldTest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toPascal(tt.input)
			assert.String(t, result).Equal(tt.expected)
		})
	}
}

func TestMatchLangVariant(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantBase    string
		wantVariant string
		wantOK      bool
	}{
		{
			name:        "zh variant",
			input:       "AGENTS.zh.md",
			wantBase:    "AGENTS.md",
			wantVariant: "zh",
			wantOK:      true,
		},
		{
			name:        "en variant",
			input:       "docs.en.md",
			wantBase:    "docs.md",
			wantVariant: "en",
			wantOK:      true,
		},
		{
			name:        "dotfile with variant",
			input:       ".env.zh.md",
			wantBase:    ".env.md",
			wantVariant: "zh",
			wantOK:      true,
		},
		{
			name:   "no variant",
			input:  "AGENTS.md",
			wantOK: false,
		},
		{
			name:   "unknown lang",
			input:  "AGENTS.fr.md",
			wantOK: false,
		},
		{
			name:   "no extension",
			input:  "AGENTS.zh",
			wantOK: false,
		},
		{
			name:   "only extension",
			input:  ".md",
			wantOK: false,
		},
		{
			name:   "empty",
			input:  "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, variant, ok := matchLangVariant(tt.input)
			assert.That(t, ok).Equal(tt.wantOK)
			assert.String(t, base).Equal(tt.wantBase)
			assert.String(t, variant).Equal(tt.wantVariant)
		})
	}
}

func TestStripLangSuffix(t *testing.T) {
	dir := t.TempDir()

	// Files at root, files in a subdir, and a symlink whose name is also a
	// lang variant. The symlink target points at the post-strip name, matching
	// the layout scaffold convention.
	mustWrite := func(rel, content string) {
		p := filepath.Join(dir, rel)
		require.Error(t, os.MkdirAll(filepath.Dir(p), 0o755)).Nil()
		require.Error(t, os.WriteFile(p, []byte(content), 0o644)).Nil()
	}
	mustWrite("AGENTS.zh.md", "zh")
	mustWrite("AGENTS.en.md", "en")
	mustWrite("README.md", "keep")
	mustWrite("docs/guide.zh.md", "zh")
	mustWrite("docs/guide.en.md", "en")
	require.Error(t, os.Symlink("AGENTS.md", filepath.Join(dir, "CLAUDE.zh.md"))).Nil()
	require.Error(t, os.Symlink("AGENTS.md", filepath.Join(dir, "CLAUDE.en.md"))).Nil()

	require.Error(t, stripLangSuffix(dir, "zh")).Nil()

	// Expected: zh variants renamed to bare names; en variants removed;
	// unrelated files untouched; symlinks renamed with target preserved.
	wantFiles := map[string]string{
		"AGENTS.md":     "zh",
		"README.md":     "keep",
		"docs/guide.md": "zh",
	}
	for rel, want := range wantFiles {
		b, err := os.ReadFile(filepath.Join(dir, rel))
		assert.Error(t, err).Nil()
		assert.String(t, string(b)).Equal(want)
	}

	wantAbsent := []string{
		"AGENTS.zh.md", "AGENTS.en.md",
		"docs/guide.zh.md", "docs/guide.en.md",
		"CLAUDE.zh.md", "CLAUDE.en.md",
	}
	for _, rel := range wantAbsent {
		_, err := os.Lstat(filepath.Join(dir, rel))
		assert.That(t, os.IsNotExist(err)).True()
	}

	target, err := os.Readlink(filepath.Join(dir, "CLAUDE.md"))
	assert.Error(t, err).Nil()
	assert.String(t, target).Equal("AGENTS.md")
}
