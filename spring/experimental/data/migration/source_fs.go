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

package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"
)

// NewFSSource builds a [Source] over the .sql files in dir within fsys. The
// canonical use is an embedded directory:
//
//	//go:embed migrations
//	var migrationsFS embed.FS
//	src := migration.NewFSSource(migrationsFS, "migrations")
//
// Files follow the Flyway-style name V<version>__<name>.sql — for example
// V1__init.sql, V2__add_users_email.sql. The leading V is optional and case
// insensitive, the separator is a double underscore, and the version is a
// non-negative integer compared numerically (so V2 precedes V10). Each file's
// content is hashed (SHA-256) into the migration's Checksum, so editing an
// already-applied file is caught by the Runner. Non-.sql files are ignored;
// subdirectories are not descended into.
func NewFSSource(fsys fs.FS, dir string) Source {
	return &fsSource{fsys: fsys, dir: dir}
}

type fsSource struct {
	fsys fs.FS
	dir  string
}

// Migrations reads and parses every .sql file in the directory. A malformed
// file name is a hard error rather than a silent skip: a script that will never
// run because its name could not be parsed is a trap, not a convenience.
func (s *fsSource) Migrations() ([]Migration, error) {
	entries, err := fs.ReadDir(s.fsys, s.dir)
	if err != nil {
		return nil, fmt.Errorf("migration: read dir %q: %w", s.dir, err)
	}
	var migs []Migration
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".sql") {
			continue
		}
		version, label, err := parseFileName(name)
		if err != nil {
			return nil, err
		}
		fp := path.Join(s.dir, name)
		content, err := fs.ReadFile(s.fsys, fp)
		if err != nil {
			return nil, fmt.Errorf("migration: read %q: %w", fp, err)
		}
		sum := sha256.Sum256(content)
		stmts := splitStatements(string(content))
		migs = append(migs, Migration{
			Version:  version,
			Name:     label,
			Checksum: hex.EncodeToString(sum[:]),
			Up:       execAll(stmts),
		})
	}
	return migs, nil
}

// parseFileName extracts the version and label from a V<version>__<name>.sql
// file name. The leading V/v is optional; the version and name are separated by
// a double underscore.
func parseFileName(name string) (uint64, string, error) {
	base := name[:len(name)-len(".sql")]
	// A trailing ".sql" in any case was already verified by the caller; strip it
	// case-insensitively.
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		base = name[:dot]
	}
	body := base
	if len(body) > 0 && (body[0] == 'V' || body[0] == 'v') {
		body = body[1:]
	}
	sep := strings.Index(body, "__")
	if sep < 0 {
		return 0, "", fmt.Errorf("migration: file %q does not match V<version>__<name>.sql (missing '__' separator)", name)
	}
	verStr, label := body[:sep], body[sep+2:]
	version, err := strconv.ParseUint(verStr, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("migration: file %q has non-numeric version %q", name, verStr)
	}
	label = strings.ReplaceAll(label, "_", " ")
	return version, label, nil
}

// execAll returns an Up that runs each statement in order through the Execer.
func execAll(stmts []string) func(ctx context.Context, exec Execer) error {
	return func(ctx context.Context, exec Execer) error {
		for _, stmt := range stmts {
			if err := exec.ExecContext(ctx, stmt); err != nil {
				return err
			}
		}
		return nil
	}
}

// splitStatements breaks a .sql file into individual statements on semicolons,
// ignoring semicolons inside single/double-quoted strings and skipping -- line
// comments. It is intentionally simple — enough for the schema DDL/DML a
// migration file carries — and deliberately not a full SQL parser: statements
// that embed procedural bodies with inner semicolons (stored procedures, PL/pgSQL
// blocks) should be issued as a single-statement file the driver executes whole.
func splitStatements(sql string) []string {
	var (
		out    []string
		buf    strings.Builder
		inS    bool // inside single quotes
		inD    bool // inside double quotes
		inLine bool // inside a -- line comment
	)
	flush := func() {
		s := strings.TrimSpace(buf.String())
		if s != "" {
			out = append(out, s)
		}
		buf.Reset()
	}
	rs := []rune(sql)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		if inLine {
			buf.WriteRune(c)
			if c == '\n' {
				inLine = false
			}
			continue
		}
		switch {
		case !inS && !inD && c == '-' && i+1 < len(rs) && rs[i+1] == '-':
			inLine = true
			buf.WriteRune(c)
		case !inD && c == '\'':
			inS = !inS
			buf.WriteRune(c)
		case !inS && c == '"':
			inD = !inD
			buf.WriteRune(c)
		case !inS && !inD && c == ';':
			flush()
		default:
			buf.WriteRune(c)
		}
	}
	flush()
	return out
}
