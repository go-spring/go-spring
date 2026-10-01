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

package loadbalance

import (
	"slices"
	"strconv"
	"time"

	"go-spring.org/stdlib/errutil"
)

// Params is the strategy-independent parameter bag a [Factory] reads its own
// knobs from: the flattened `balancer-params` map of a selection rule, handed
// over untouched. The core never interprets a key — a strategy owns the keys it
// reads and reports the rest through [Params.Done]. That is what lets a
// strategy be extended with parameters of its own without any change here.
//
// Reading a key marks it consumed, which preserves the strict-partition
// guarantee the typed Config used to give: a parameter aimed at another
// strategy is a construction error, never a silently dropped value.
//
// A Params is a one-shot reader owned by the factory that receives it; it is
// not safe for concurrent use and must not be retained past construction.
type Params struct {
	vals map[string]string
	used map[string]bool
}

// NewParams wraps vals as a readable [Params]. A nil map is the empty bag, so a
// strategy can be built from a rule that names it without parameters.
func NewParams(vals map[string]string) *Params {
	return &Params{vals: vals, used: map[string]bool{}}
}

// String returns the value of key, or def when key is unset. It is the one
// primitive that marks a key consumed; [Params.Int] and [Params.Duration] are
// typed decoders built on it, and a strategy that needs another shape reads the
// raw string and parses it itself.
func (p *Params) String(key, def string) string {
	p.used[key] = true
	if v, ok := p.vals[key]; ok {
		return v
	}
	return def
}

// Int returns key parsed as an integer, or def when key is unset. A value that
// is present but not an integer is an error, so a typo never falls back to the
// default silently.
func (p *Params) Int(key string, def int) (int, error) {
	s := p.String(key, "")
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, errutil.Explain(err, "loadbalance: parameter %q is not an integer", key)
	}
	return n, nil
}

// Duration returns key parsed as a [time.Duration] ("250ms", "3s"), or def when
// key is unset. A present-but-unparsable value is an error.
func (p *Params) Duration(key string, def time.Duration) (time.Duration, error) {
	s := p.String(key, "")
	if s == "" {
		return def, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, errutil.Explain(err, "loadbalance: parameter %q is not a duration", key)
	}
	return d, nil
}

// Done reports an error naming the keys that were never read. Every factory
// ends with it, so a parameter aimed at another strategy fails construction
// instead of being silently ignored.
func (p *Params) Done() error {
	var left []string
	for k := range p.vals {
		if !p.used[k] {
			left = append(left, k)
		}
	}
	if len(left) == 0 {
		return nil
	}
	slices.Sort(left)
	return errutil.Explain(nil, "loadbalance: parameter(s) not understood by this strategy: %v", left)
}
