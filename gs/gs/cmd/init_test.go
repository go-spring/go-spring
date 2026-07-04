package cmd

import (
	"os"
	"path/filepath"
	"testing"
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
			if result != tt.expected {
				t.Errorf("toPascal(%q) = %q, want %q", tt.input, result, tt.expected)
			}
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
			if ok != tt.wantOK || base != tt.wantBase || variant != tt.wantVariant {
				t.Errorf("matchLangVariant(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.input, base, variant, ok, tt.wantBase, tt.wantVariant, tt.wantOK)
			}
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
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("AGENTS.zh.md", "zh")
	mustWrite("AGENTS.en.md", "en")
	mustWrite("README.md", "keep")
	mustWrite("docs/guide.zh.md", "zh")
	mustWrite("docs/guide.en.md", "en")
	if err := os.Symlink("AGENTS.md", filepath.Join(dir, "CLAUDE.zh.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("AGENTS.md", filepath.Join(dir, "CLAUDE.en.md")); err != nil {
		t.Fatal(err)
	}

	if err := stripLangSuffix(dir, "zh"); err != nil {
		t.Fatalf("stripLangSuffix: %v", err)
	}

	// Expected: zh variants renamed to bare names; en variants removed;
	// unrelated files untouched; symlinks renamed with target preserved.
	wantFiles := map[string]string{
		"AGENTS.md":     "zh",
		"README.md":     "keep",
		"docs/guide.md": "zh",
	}
	for rel, want := range wantFiles {
		b, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Errorf("read %q: %v", rel, err)
			continue
		}
		if string(b) != want {
			t.Errorf("%q content = %q, want %q", rel, b, want)
		}
	}

	wantAbsent := []string{
		"AGENTS.zh.md", "AGENTS.en.md",
		"docs/guide.zh.md", "docs/guide.en.md",
		"CLAUDE.zh.md", "CLAUDE.en.md",
	}
	for _, rel := range wantAbsent {
		if _, err := os.Lstat(filepath.Join(dir, rel)); !os.IsNotExist(err) {
			t.Errorf("expected %q to be absent, err=%v", rel, err)
		}
	}

	target, err := os.Readlink(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("readlink CLAUDE.md: %v", err)
	}
	if target != "AGENTS.md" {
		t.Errorf("CLAUDE.md target = %q, want %q", target, "AGENTS.md")
	}
}
