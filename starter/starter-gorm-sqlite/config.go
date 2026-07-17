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

// Config holds the configuration parameters for a SQLite connection.
// SQLite is file/in-memory based — a single DSN is enough to describe the
// data source (e.g. "test.db", ":memory:", or "file:primary?mode=memory&cache=shared").
type Config struct {
	Dsn string `value:"${dsn}"` // SQLite data source (file path or SQLite URI)
}

// DSN returns the SQLite data source string. Kept as a method so the
// newClient signature stays identical to the other gorm starters.
func (c Config) DSN() string {
	return c.Dsn
}
