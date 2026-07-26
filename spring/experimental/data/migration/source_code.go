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

// codeSource is a [Source] whose migrations are registered in Go code. It is
// the shape to use when a change is more than a static .sql file can express —
// e.g. it must read a row, transform it, and write it back — since the Up
// closure has the full [Execer] at hand.
type codeSource struct {
	migs []Migration
}

// NewSource returns a [Source] over migrations declared in code. The Runner
// sorts and validates them, so the argument order is irrelevant. A migration
// registered this way may leave Checksum empty (opting out of the tamper check)
// or set it to a stable value the caller controls.
func NewSource(migs ...Migration) Source {
	cp := make([]Migration, len(migs))
	copy(cp, migs)
	return &codeSource{migs: cp}
}

// Migrations returns a copy of the registered migrations so a caller mutating
// the slice cannot corrupt the source's own state.
func (s *codeSource) Migrations() ([]Migration, error) {
	out := make([]Migration, len(s.migs))
	copy(out, s.migs)
	return out, nil
}
