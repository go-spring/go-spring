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

package reader

import (
	"os"
	"path/filepath"
	"strings"

	"go-spring.org/spring/conf/reader/json"
	"go-spring.org/spring/conf/reader/prop"
	"go-spring.org/spring/conf/reader/toml"
	"go-spring.org/spring/conf/reader/yaml"
	"go-spring.org/stdlib/errutil"
)

var readers = map[string]Reader{}

func init() {
	Register(json.Read, ".json")
	Register(prop.Read, ".properties", ".props")
	Register(yaml.Read, ".yaml", ".yml")
	Register(toml.Read, ".toml", ".tml")
}

// Reader parses raw bytes into a nested map[string]any.
type Reader func(b []byte) (map[string]any, error)

// Register registers its Reader for some kind of file extension.
// Must be called in init functions only.
func Register(r Reader, ext ...string) {
	if r == nil {
		panic("reader cannot be nil")
	}
	for _, s := range ext {
		if s == "" {
			panic("file extension cannot be empty")
		}
		if _, ok := readers[s]; ok {
			panic("file extension " + s + " has been registered")
		}
		readers[s] = r
	}
}

// Read parses b according to format, which may be a format name ("yaml") or an
// extension (".yaml") — they are the same registry key, with or without the
// leading dot. Use ReadFile when the bytes come from a file on disk.
func Read(format string, b []byte) (map[string]any, error) {
	r, ok := lookup(format)
	if !ok {
		return nil, errutil.Explain(nil, "unsupported config format %q", format)
	}
	return r(b)
}

// Has reports whether a reader is registered for format (name or extension).
// Use it to validate or probe a format without parsing.
func Has(format string) bool {
	_, ok := lookup(format)
	return ok
}

// lookup resolves format (name or extension, with or without the dot) to a
// registered Reader.
func lookup(format string) (Reader, bool) {
	if format != "" && !strings.HasPrefix(format, ".") {
		format = "." + format
	}
	r, ok := readers[format]
	return r, ok
}

// ReadFile reads a file and parses its content based on file extension.
// Returns an error if the file cannot be read or the file type is unsupported.
func ReadFile(file string) (map[string]any, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return Read(filepath.Ext(file), b)
}
