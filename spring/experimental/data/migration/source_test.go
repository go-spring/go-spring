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
	"testing/fstest"

	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestFSSource_ParsesAndHashes(t *testing.T) {
	fsys := fstest.MapFS{
		"mig/V1__init.sql":          {Data: []byte("CREATE TABLE t (id INT);")},
		"mig/V10__add_email.sql":    {Data: []byte("ALTER TABLE t ADD email TEXT;")},
		"mig/v2__second_change.sql": {Data: []byte("ALTER TABLE t ADD age INT;")},
		"mig/notes.txt":             {Data: []byte("ignored")},
		"mig/sub/V3__nested.sql":    {Data: []byte("SELECT 1;")}, // subdir ignored
	}
	migs, err := NewFSSource(fsys, "mig").Migrations()
	assert.Error(t, err).Nil()
	assert.That(t, len(migs)).Equal(3)

	// Runner sorts, but the source order is directory order; verify by indexing.
	byVer := map[uint64]Migration{}
	for _, m := range migs {
		byVer[m.Version] = m
	}
	assert.That(t, byVer[1].Name).Equal("init")
	assert.That(t, byVer[2].Name).Equal("second change") // underscores become spaces
	assert.That(t, byVer[10].Name).Equal("add email")
	// Checksums are non-empty and differ per file.
	assert.That(t, byVer[1].Checksum != "").True()
	assert.That(t, byVer[1].Checksum != byVer[2].Checksum).True()
}

func TestFSSource_BadNameIsHardError(t *testing.T) {
	fsys := fstest.MapFS{"mig/oops.sql": {Data: []byte("SELECT 1;")}}
	_, err := NewFSSource(fsys, "mig").Migrations()
	assert.Error(t, err).Matches("missing '__' separator")

	fsys2 := fstest.MapFS{"mig/Vxx__bad.sql": {Data: []byte("SELECT 1;")}}
	_, err = NewFSSource(fsys2, "mig").Migrations()
	assert.Error(t, err).Matches("non-numeric version")
}

func TestFSSource_UpRunsEachStatement(t *testing.T) {
	fsys := fstest.MapFS{
		"mig/V1__multi.sql": {Data: []byte("CREATE TABLE t (id INT);\nINSERT INTO t VALUES (1);\n")},
	}
	migs, err := NewFSSource(fsys, "mig").Migrations()
	assert.Error(t, err).Nil()

	var got []string
	err = migs[0].Up(context.Background(), execerFunc(func(_ context.Context, q string, _ ...any) error {
		got = append(got, q)
		return nil
	}))
	assert.Error(t, err).Nil()
	assert.That(t, len(got)).Equal(2)
}

func TestSplitStatements(t *testing.T) {
	// Semicolons inside quotes and -- comments are not statement boundaries.
	sql := "INSERT INTO t VALUES ('a;b'); -- trailing; comment\nSELECT 1;"
	stmts := splitStatements(sql)
	assert.That(t, len(stmts)).Equal(2)
	assert.That(t, stmts[0]).Equal("INSERT INTO t VALUES ('a;b')")

	// A file with no trailing semicolon still yields its single statement.
	assert.That(t, splitStatements("SELECT 1")).Equal([]string{"SELECT 1"})
	// Empty / whitespace-only / bare-semicolon input yields no statements.
	assert.That(t, len(splitStatements("  \n ;; "))).Equal(0)
}

type execerFunc func(ctx context.Context, query string, args ...any) error

func (f execerFunc) ExecContext(ctx context.Context, query string, args ...any) error {
	return f(ctx, query, args...)
}
