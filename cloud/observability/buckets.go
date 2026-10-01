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

package observability

import "slices"

// DurationBuckets returns the duration-histogram boundaries, in seconds, that
// every component builds its duration histograms from: the OTel HTTP semconv
// recommended set. One definition rather than a literal per package, for the
// same reason log keys and metric attribute names have one: a boundary is what
// makes two histograms comparable, and a drifted boundary is silent — the
// metric still reports, it just can no longer be bucketed against its siblings.
//
// It returns a fresh copy on every call. A component whose durations live on a
// different scale (cloud/scheduling's wake-up lag is the one such case today)
// takes the copy, edits it, and keeps it local; it cannot retune every other
// component's histograms by writing through this name.
func DurationBuckets() []float64 { return slices.Clone(durationBuckets[:]) }

// durationBuckets is the single definition [DurationBuckets] hands out. An
// array, not a slice literal, so passing it whole copies rather than aliases.
var durationBuckets = [14]float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}
