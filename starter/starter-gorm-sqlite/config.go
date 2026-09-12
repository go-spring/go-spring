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

package StarterGormSqlite

import (
	"fmt"
	"strings"

	"go-spring.org/starter-gorm"
)

// Config holds the configuration parameters for a SQLite connection. The
// shared pool settings come from the embedded gormcore.PoolSettings — sqlite
// is an in-process engine, so the discovery keys of gormcore.Common are
// deliberately not bound here. The fields below are the SQLite-specific
// connection parameters.
type Config struct {
	gormcore.PoolSettings

	// ObserveEnabled is the observe-plugin kill switch, identical in meaning to
	// the one in gormcore.Common.
	ObserveEnabled bool `value:"${observe.enabled:=true}"`

	// File is the SQLite database path, or ":memory:" for a per-connection
	// in-memory database. Required. The URI is built from it via DSN().
	File string `value:"${file}" expr:"$ != ''"`

	// JournalMode sets the SQLite journal mode (wal, delete, truncate,
	// persist, memory, off). Default "wal". Appended as a _pragma.
	JournalMode string `value:"${journal-mode:=wal}"`

	// BusyTimeout is the SQLite busy timeout in milliseconds, applied as a
	// _busy_timeout pragma so concurrent writers wait rather than fail with
	// SQLITE_BUSY. Default 5000.
	BusyTimeout int `value:"${busy-timeout:=5000}"`

	// ForeignKeys enables the foreign_keys pragma (1 = on). Default true, the
	// common expectation for a relational database.
	ForeignKeys bool `value:"${foreign-keys:=true}"`
}

// DSN builds the modernc.org/sqlite URI the dialect opens. Pragmas ride the
// query string as documented by the driver (file?_pragma=busy_timeout(5000));
// each value is parenthesized, so a future key-only pragma must be added by
// hand rather than through this builder.
func (c Config) DSN() string {
	var pragmas []string
	if c.JournalMode != "" {
		pragmas = append(pragmas, fmt.Sprintf("_pragma=journal_mode(%s)", c.JournalMode))
	}
	if c.BusyTimeout > 0 {
		pragmas = append(pragmas, fmt.Sprintf("_pragma=busy_timeout(%d)", c.BusyTimeout))
	}
	if c.ForeignKeys {
		pragmas = append(pragmas, "_pragma=foreign_keys(1)")
	}

	file := c.File
	if file == ":memory:" {
		file = ":memory:"
	}
	if len(pragmas) == 0 {
		return file
	}
	return file + "?" + strings.Join(pragmas, "&")
}
