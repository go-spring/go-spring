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
	"fmt"
	"sort"
)

// Options tunes how the [Runner] applies migrations.
type Options struct {
	// AllowOutOfOrder permits applying a pending migration whose version is
	// lower than the highest already-applied version — an insert into a gap
	// after later versions ran. Default false: history must extend strictly
	// upward, so a late-arriving low version fails fast rather than applying in
	// a surprising order.
	AllowOutOfOrder bool

	// Baseline marks every migration with Version <= Baseline as applied
	// without running its Up. It is for adopting the framework on a database
	// that already carries schema up to some version: those scripts are recorded
	// as a starting point, not executed. 0 disables baselining.
	Baseline uint64
}

// Runner is the core executor. It is safe for a single sequential Migrate call
// at startup; concurrent Migrate calls against one Runner are not supported
// (cross-process safety comes from the Store's atomic Apply and the version
// table's primary key, not from this type).
type Runner struct {
	store  Store
	source Source
	opts   Options
}

// NewRunner builds a Runner over a backend [Store] and a [Source].
func NewRunner(store Store, source Source, opts Options) *Runner {
	return &Runner{store: store, source: source, opts: opts}
}

// Migrate applies every pending migration in ascending version order and
// returns the migrations it applied (excluding ones skipped as already-applied
// or recorded via baseline). It is the single entry point a starter calls at
// startup; any error aborts and is returned unwrapped enough to fail the boot.
//
// The sequence is:
//
//  1. ensure the version table exists;
//  2. load and validate the source (positive, unique, sorted versions);
//  3. read applied records and detect checksum drift on already-applied ones;
//  4. for each pending migration: record it as baseline if <= Baseline, reject
//     it if it is out of order and AllowOutOfOrder is false, otherwise apply it.
func (r *Runner) Migrate(ctx context.Context) ([]Migration, error) {
	if err := r.store.EnsureVersionTable(ctx); err != nil {
		return nil, fmt.Errorf("migration: ensure version table: %w", err)
	}

	migs, err := r.loadSorted()
	if err != nil {
		return nil, err
	}

	recs, err := r.store.AppliedRecords(ctx)
	if err != nil {
		return nil, fmt.Errorf("migration: read applied records: %w", err)
	}
	applied := make(map[uint64]Record, len(recs))
	var maxApplied uint64
	for _, rec := range recs {
		applied[rec.Version] = rec
		if rec.Version > maxApplied {
			maxApplied = rec.Version
		}
	}

	var done []Migration
	for _, m := range migs {
		if rec, ok := applied[m.Version]; ok {
			// Already applied: guard against an edited historical script.
			if m.Checksum != "" && rec.Checksum != "" && m.Checksum != rec.Checksum {
				return done, fmt.Errorf(
					"migration: checksum mismatch for version %d (%q): recorded %q but source is %q — a migration already applied was edited; migrations are immutable history",
					m.Version, m.Name, rec.Checksum, m.Checksum)
			}
			continue
		}

		// Pending migration.
		if m.Version <= r.opts.Baseline {
			if err := r.store.MarkApplied(ctx, m); err != nil {
				return done, fmt.Errorf("migration: baseline version %d (%q): %w", m.Version, m.Name, err)
			}
			continue
		}
		if !r.opts.AllowOutOfOrder && m.Version < maxApplied {
			return done, fmt.Errorf(
				"migration: version %d (%q) is below the highest applied version %d — out-of-order migration; set allow-out-of-order to permit gap fills",
				m.Version, m.Name, maxApplied)
		}
		if err := r.store.Apply(ctx, m); err != nil {
			return done, fmt.Errorf("migration: apply version %d (%q): %w", m.Version, m.Name, err)
		}
		done = append(done, m)
		if m.Version > maxApplied {
			maxApplied = m.Version
		}
	}
	return done, nil
}

// loadSorted fetches the source's migrations, sorts them by version ascending,
// and validates that every version is positive and unique.
func (r *Runner) loadSorted() ([]Migration, error) {
	migs, err := r.source.Migrations()
	if err != nil {
		return nil, fmt.Errorf("migration: load source: %w", err)
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].Version < migs[j].Version })
	for i, m := range migs {
		if m.Version == 0 {
			return nil, fmt.Errorf("migration: version must be > 0 (migration %q)", m.Name)
		}
		if i > 0 && migs[i-1].Version == m.Version {
			return nil, fmt.Errorf("migration: duplicate version %d (%q and %q)",
				m.Version, migs[i-1].Name, m.Name)
		}
	}
	return migs, nil
}
