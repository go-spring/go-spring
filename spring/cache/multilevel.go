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

package cache

import (
	"context"
	"errors"
	"time"
)

// MultiLevel composes several caches into a near-to-far hierarchy (e.g. an
// in-process [Memory] level in front of a shared Redis level). It is the
// multi-level cache equivalent: the near level absorbs most reads at process
// speed while the far level keeps replicas coherent.
//
// Reads probe levels from near (index 0) to far; the first hit is returned and
// the value is written back into every nearer level that missed (read-through
// backfill) so subsequent reads stay local. Writes and deletes fan out to every
// level so the whole hierarchy stays consistent. A per-level read error is
// tolerated (the next level is tried); write/delete errors from all levels are
// joined.
type MultiLevel struct {
	levels      []Cache
	backfillTTL time.Duration
}

// NewMultiLevel builds a multi-level cache over levels, ordered near-to-far
// (index 0 is consulted first and is where backfill lands). backfillTTL is the
// ttl used when a far hit is written back into nearer levels; a non-positive
// value backfills without expiry, which is rarely what you want for a bounded
// near cache, so pass the near level's intended ttl. It panics if no levels are
// given, since an empty hierarchy can never serve a value.
func NewMultiLevel(backfillTTL time.Duration, levels ...Cache) *MultiLevel {
	if len(levels) == 0 {
		panic("cache: NewMultiLevel requires at least one level")
	}
	return &MultiLevel{levels: levels, backfillTTL: backfillTTL}
}

// Get implements [Cache]. It returns the first hit found scanning near-to-far
// and backfills nearer levels that missed. A level's error does not abort the
// scan: a nearer level failing must not hide a value the far level still holds.
// If no level has the key, it returns found=false with the last error seen (if
// any), so a total backend outage still surfaces.
func (m *MultiLevel) Get(ctx context.Context, key string) (any, bool, error) {
	var lastErr error
	for i, lvl := range m.levels {
		v, ok, err := lvl.Get(ctx, key)
		if err != nil {
			lastErr = err
			continue
		}
		if !ok {
			continue
		}
		// Backfill the nearer levels that missed; best-effort, since a failed
		// backfill only costs a future far read, not correctness.
		for j := range i {
			_ = m.levels[j].Set(ctx, key, v, m.backfillTTL)
		}
		return v, true, nil
	}
	return nil, false, lastErr
}

// Set implements [Cache] by writing through to every level with the same ttl.
func (m *MultiLevel) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	var errs []error
	for _, lvl := range m.levels {
		if err := lvl.Set(ctx, key, val, ttl); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Delete implements [Cache] by removing key from every level.
func (m *MultiLevel) Delete(ctx context.Context, key string) error {
	var errs []error
	for _, lvl := range m.levels {
		if err := lvl.Delete(ctx, key); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
