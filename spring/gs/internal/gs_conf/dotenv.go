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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// EnvFile defines the environment variable name used to override the
// default .env file location. This allows users to load environment
// variables from a non-standard path.
//
// The default location is ".env" in the current working directory.
const EnvFile = "GS_ENV_FILE"

// loadDotEnv reads a .env file and applies its KEY=VALUE pairs to the
// process environment. Variables that are already set in the OS
// environment are not overridden, so real environment variables always
// take precedence over values declared in the file.
//
// The file location defaults to ".env" in the current working directory
// and can be overridden via the GS_ENV_FILE environment variable.
//
// A missing file is silently skipped; any other read or parse error is
// returned. Loaded variables afterwards flow through extractEnvironments
// exactly like ordinary environment variables: GS_-prefixed keys are
// mapped to dotted properties (GS_DB_HOST -> db.host) while all other
// keys are kept as-is.
func loadDotEnv() error {
	filename := strings.TrimSpace(os.Getenv(EnvFile))
	if filename == "" {
		filename = ".env"
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	pairs, err := parseDotEnv(data)
	if err != nil {
		return err
	}
	// Deduplicate with last-wins so the values applied to the process
	// environment match the ones parseDotEnv uses for in-file expansion.
	// Real OS environment variables are never overridden.
	values := make(map[string]string, len(pairs))
	for _, kv := range pairs {
		values[kv.key] = kv.value
	}
	for k, v := range values {
		if _, ok := os.LookupEnv(k); ok {
			continue // do not override an existing environment variable
		}
		_ = os.Setenv(k, v)
	}
	return nil
}

// envPair is a single KEY=VALUE entry parsed from a .env file.
type envPair struct {
	key   string
	value string
}

// parseDotEnv parses the contents of a .env file into an ordered list of
// KEY=VALUE pairs.
//
// Supported syntax (compatible with the widely used dotenv format):
//
//   - Blank lines and lines starting with '#' are ignored.
//   - An optional "export " prefix may precede the key.
//   - Keys must be valid environment variable names ([A-Za-z_][A-Za-z0-9_]*).
//   - Values may be unquoted, double-quoted, or single-quoted.
//   - Double-quoted values may span multiple lines and interpret backslash
//     escapes (\n, \r, \t, \", \\, \$, \<newline>) and variable references
//     ($VAR, ${VAR}).
//   - Single-quoted values are taken literally (no escapes, no expansion)
//     and may also span multiple lines.
//   - Variable references resolve against values declared earlier in the
//     file, falling back to the OS environment. References whose name is
//     not a valid environment variable name (e.g. ${a.b}) are left untouched
//     so that Go-Spring's own placeholder resolution can handle them later.
//
// A leading UTF-8 BOM (written by some Windows/Mac editors) is tolerated.
//
// Example:
//
//	# a comment line
//	export GS_DB_HOST=localhost
//	GS_DB_PORT=5432
//	DB_URL="postgres://$GS_DB_HOST:$GS_DB_PORT/db"
//	SECRET='pass$word'             # single-quoted: $word stays literal
//	MSG="Hello ${spring.app.name}" # left for Go-Spring to resolve
func parseDotEnv(data []byte) ([]envPair, error) {
	// Tolerate a leading UTF-8 BOM written by some Windows/Mac editors.
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}
	p := &envParser{data: data, line: 1}
	var pairs []envPair
	lookup := func(name string) (string, bool) {
		for i := len(pairs) - 1; i >= 0; i-- {
			if pairs[i].key == name {
				return pairs[i].value, true
			}
		}
		return os.LookupEnv(name)
	}
	for {
		p.skipIgnored()
		if p.eof() {
			break
		}
		key, err := p.readKey()
		if err != nil {
			return nil, err
		}
		value, err := p.readValue(lookup)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, envPair{key: key, value: value})
	}
	return pairs, nil
}

// envParser is a tiny byte scanner over a .env file's contents.
type envParser struct {
	data []byte
	pos  int
	line int
}

func (p *envParser) eof() bool { return p.pos >= len(p.data) }

func (p *envParser) peek() byte {
	if p.eof() {
		return 0
	}
	return p.data[p.pos]
}

func (p *envParser) next() byte {
	b := p.data[p.pos]
	p.pos++
	if b == '\n' {
		p.line++
	}
	return b
}

func (p *envParser) hasPrefix(s string) bool {
	return p.pos+len(s) <= len(p.data) && string(p.data[p.pos:p.pos+len(s)]) == s
}

// skipIgnored consumes whitespace, blank lines and comment lines.
func (p *envParser) skipIgnored() {
	for !p.eof() {
		switch p.peek() {
		case ' ', '\t', '\r', '\n':
			p.next()
		case '#':
			for !p.eof() && p.peek() != '\n' {
				p.next()
			}
		default:
			return
		}
	}
}

// hasExportPrefix reports whether the parser is positioned at the
// bash-style "export " prefix.
func (p *envParser) hasExportPrefix() bool {
	const prefix = "export"
	if !p.hasPrefix(prefix) {
		return false
	}
	i := p.pos + len(prefix)
	return i < len(p.data) && (p.data[i] == ' ' || p.data[i] == '\t')
}

// readKey reads the variable name up to '=' and consumes the '='.
func (p *envParser) readKey() (string, error) {
	if p.hasExportPrefix() {
		p.pos += len("export")
		for p.peek() == ' ' || p.peek() == '\t' {
			p.next()
		}
	}
	start := p.pos
	for !p.eof() {
		b := p.peek()
		if b == '=' || b == '\n' || b == '#' {
			break
		}
		p.next()
	}
	key := strings.TrimRight(string(p.data[start:p.pos]), " \t")
	if p.peek() != '=' {
		return "", fmt.Errorf(".env line %d: missing '=' after key %q", p.line, key)
	}
	p.next() // consume '='
	if key == "" {
		return "", fmt.Errorf(".env line %d: empty variable name", p.line)
	}
	if !isValidEnvName(key) {
		return "", fmt.Errorf(".env line %d: invalid variable name %q", p.line, key)
	}
	return key, nil
}

// readValue reads the value following '='. Leading whitespace is skipped.
// Quoted values may span multiple lines; unquoted values end at the line.
func (p *envParser) readValue(lookup func(string) (string, bool)) (string, error) {
	for p.peek() == ' ' || p.peek() == '\t' {
		p.next()
	}
	if p.eof() {
		return "", nil
	}
	switch p.peek() {
	case '"':
		return p.readDoubleQuoted(lookup)
	case '\'':
		return p.readSingleQuoted()
	default:
		return p.readUnquoted(lookup)
	}
}

// readUnquoted reads a value up to the end of the line. Variable references
// are expanded; surrounding whitespace is trimmed. The '#' character is
// treated as part of the value (no inline comments).
func (p *envParser) readUnquoted(lookup func(string) (string, bool)) (string, error) {
	var b strings.Builder
	for !p.eof() && p.peek() != '\n' {
		if p.peek() == '$' {
			p.next() // consume '$'
			p.expandVar(&b, lookup)
			continue
		}
		b.WriteByte(p.next())
	}
	return strings.TrimRight(b.String(), " \t\r"), nil
}

// readSingleQuoted reads a literal value wrapped in single quotes. No
// escapes or variable expansion are performed.
func (p *envParser) readSingleQuoted() (string, error) {
	p.next() // opening quote
	var b strings.Builder
	for !p.eof() {
		c := p.peek()
		if c == '\'' {
			p.next() // closing quote
			return b.String(), p.expectLineEnd()
		}
		b.WriteByte(p.next())
	}
	return "", fmt.Errorf(".env line %d: unterminated single-quoted value", p.line)
}

// readDoubleQuoted reads a value wrapped in double quotes. Backslash
// escapes and variable references are interpreted and the value may span
// multiple lines.
func (p *envParser) readDoubleQuoted(lookup func(string) (string, bool)) (string, error) {
	p.next() // opening quote
	var b strings.Builder
	for !p.eof() {
		switch p.peek() {
		case '"':
			p.next() // closing quote
			return b.String(), p.expectLineEnd()
		case '\\':
			p.next() // consume backslash
			if p.eof() {
				return "", fmt.Errorf(".env line %d: unterminated double-quoted value", p.line)
			}
			p.writeEscape(&b)
		case '$':
			p.next() // consume '$'
			p.expandVar(&b, lookup)
		default:
			b.WriteByte(p.next())
		}
	}
	return "", fmt.Errorf(".env line %d: unterminated double-quoted value", p.line)
}

// writeEscape writes the escape sequence following a backslash to b.
// The backslash has already been consumed.
func (p *envParser) writeEscape(b *strings.Builder) {
	switch e := p.next(); e {
	case 'n':
		b.WriteByte('\n')
	case 'r':
		b.WriteByte('\r')
	case 't':
		b.WriteByte('\t')
	case '"':
		b.WriteByte('"')
	case '\\':
		b.WriteByte('\\')
	case '$':
		b.WriteByte('$')
	case '\n':
		// line continuation: join with the next line, emit nothing
	default:
		// unknown escape: keep both the backslash and the character
		b.WriteByte('\\')
		b.WriteByte(e)
	}
}

// expandVar reads a variable reference starting at the current position
// (the leading '$' has already been consumed) and appends its expanded
// value to b. References whose name is not a valid environment variable
// name (e.g. ${a.b}) are appended literally so Go-Spring can resolve them.
func (p *envParser) expandVar(b *strings.Builder, lookup func(string) (string, bool)) {
	if p.eof() {
		b.WriteByte('$')
		return
	}
	if p.peek() == '{' {
		p.next() // consume '{'
		start := p.pos
		for !p.eof() && p.peek() != '}' && p.peek() != '\n' {
			p.next()
		}
		if p.eof() || p.peek() == '\n' {
			// no closing brace: keep everything literal
			b.WriteByte('$')
			b.WriteByte('{')
			b.Write(p.data[start:p.pos])
			return
		}
		name := string(p.data[start:p.pos])
		p.next() // consume '}'
		if !isValidEnvName(name) {
			b.WriteByte('$')
			b.WriteByte('{')
			b.WriteString(name)
			b.WriteByte('}')
			return
		}
		v, _ := lookup(name)
		b.WriteString(v)
		return
	}
	if isNameStart(p.peek()) {
		start := p.pos
		for !p.eof() && isNameChar(p.peek()) {
			p.next()
		}
		name := string(p.data[start:p.pos])
		v, _ := lookup(name)
		b.WriteString(v)
		return
	}
	// '$' not followed by a name: literal dollar
	b.WriteByte('$')
}

// expectLineEnd verifies that only whitespace or a trailing comment
// remains after a closing quote on the current line.
func (p *envParser) expectLineEnd() error {
	for p.peek() == ' ' || p.peek() == '\t' {
		p.next()
	}
	switch p.peek() {
	case 0, '\n', '\r':
		return nil
	case '#':
		for !p.eof() && p.peek() != '\n' {
			p.next()
		}
		return nil
	default:
		return fmt.Errorf(".env line %d: unexpected content after quoted value", p.line)
	}
}

// isValidEnvName reports whether s is a valid environment variable name:
// [A-Za-z_][A-Za-z0-9_]*.
func isValidEnvName(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			continue
		}
		if i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return true
}

func isNameStart(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isNameChar(b byte) bool {
	return isNameStart(b) || (b >= '0' && b <= '9')
}
