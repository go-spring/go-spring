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

package StarterGateway

import (
	"net/http"
	"strings"
	"time"
)

// buildPredicates turns a RouteRaw's predicate literals into a slice of
// Predicates. They are AND-combined by Route.match. An empty RouteRaw yields no
// predicates (a catch-all route). It returns an error for malformed literals so
// a bad edit is rejected at compile time and never swapped in.
func buildPredicates(raw RouteRaw) ([]Predicate, error) {
	var ps []Predicate

	if raw.Path != "" {
		ps = append(ps, pathPredicate(raw.Path))
	}
	if raw.Methods != "" {
		ps = append(ps, methodPredicate(raw.Methods))
	}
	if raw.Host != "" {
		ps = append(ps, hostPredicate(raw.Host))
	}
	if raw.Headers != "" {
		p, err := headerPredicate(raw.Headers)
		if err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	if raw.Queries != "" {
		p, err := queryPredicate(raw.Queries)
		if err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	if raw.After != "" {
		t, err := time.Parse(time.RFC3339, raw.After)
		if err != nil {
			return nil, err
		}
		ps = append(ps, afterPredicate(t))
	}
	return ps, nil
}

// pathPredicate matches an ant-style path pattern: "**" matches any number of
// segments (including zero) and "*" matches within a single segment. A trailing
// "/**" therefore matches the prefix and everything under it.
func pathPredicate(pattern string) Predicate {
	return func(r *http.Request) bool {
		return antMatch(pattern, r.URL.Path)
	}
}

// methodPredicate matches any of the comma-separated HTTP methods.
func methodPredicate(methods string) Predicate {
	set := map[string]struct{}{}
	for _, m := range strings.Split(methods, ",") {
		if m = strings.ToUpper(strings.TrimSpace(m)); m != "" {
			set[m] = struct{}{}
		}
	}
	return func(r *http.Request) bool {
		_, ok := set[r.Method]
		return ok
	}
}

// hostPredicate matches the request Host either exactly or, for a "*.suffix"
// pattern, by domain suffix.
func hostPredicate(pattern string) Predicate {
	pattern = strings.ToLower(pattern)
	suffix, wildcard := strings.CutPrefix(pattern, "*.")
	return func(r *http.Request) bool {
		host := strings.ToLower(r.Host)
		if i := strings.IndexByte(host, ':'); i >= 0 {
			host = host[:i]
		}
		if wildcard {
			return host == suffix || strings.HasSuffix(host, "."+suffix)
		}
		return host == pattern
	}
}

// headerPredicate matches when every "K:V" pair is present with the exact value.
func headerPredicate(spec string) (Predicate, error) {
	pairs, err := parsePairs(spec, ':')
	if err != nil {
		return nil, err
	}
	return func(r *http.Request) bool {
		for k, v := range pairs {
			if r.Header.Get(k) != v {
				return false
			}
		}
		return true
	}, nil
}

// queryPredicate matches when every "k=v" query parameter is present with the
// exact value.
func queryPredicate(spec string) (Predicate, error) {
	pairs, err := parsePairs(spec, '=')
	if err != nil {
		return nil, err
	}
	return func(r *http.Request) bool {
		q := r.URL.Query()
		for k, v := range pairs {
			if q.Get(k) != v {
				return false
			}
		}
		return true
	}, nil
}

// afterPredicate matches only requests received at or after t.
func afterPredicate(t time.Time) Predicate {
	return func(r *http.Request) bool {
		return !time.Now().Before(t)
	}
}

// parsePairs splits a "k<sep>v;k2<sep>v2" spec into a map, trimming spaces.
func parsePairs(spec string, sep byte) (map[string]string, error) {
	out := map[string]string{}
	for _, part := range strings.Split(spec, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		i := strings.IndexByte(part, sep)
		if i < 0 {
			return nil, &parseError{what: "predicate pair", token: part}
		}
		k := strings.TrimSpace(part[:i])
		v := strings.TrimSpace(part[i+1:])
		if k == "" {
			return nil, &parseError{what: "predicate key", token: part}
		}
		out[k] = v
	}
	return out, nil
}

// antMatch implements ant-style path matching for "*" (single segment) and "**"
// (any number of segments). It splits on "/" and walks the two patterns with a
// small recursive matcher; route paths are short so this is not a hot spot.
func antMatch(pattern, path string) bool {
	if pattern == "" {
		return path == ""
	}
	if pattern == "/**" {
		return true
	}
	return segMatch(splitSeg(pattern), splitSeg(path))
}

func splitSeg(s string) []string {
	s = strings.Trim(s, "/")
	if s == "" {
		return nil
	}
	return strings.Split(s, "/")
}

// segMatch matches path segments against pattern segments. "**" consumes zero or
// more segments; "*" and literal segments consume exactly one.
func segMatch(pat, path []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			if len(pat) == 1 {
				return true // trailing ** matches the rest
			}
			// Try to match the remainder of the pattern at each position.
			for i := 0; i <= len(path); i++ {
				if segMatch(pat[1:], path[i:]) {
					return true
				}
			}
			return false
		}
		if len(path) == 0 {
			return false
		}
		if pat[0] != "*" && !singleMatch(pat[0], path[0]) {
			return false
		}
		pat, path = pat[1:], path[1:]
	}
	return len(path) == 0
}

// singleMatch matches one segment, honoring a "*" wildcard inside the segment
// (e.g. "v*" matches "v1"). Without "*" it is an exact compare.
func singleMatch(pat, seg string) bool {
	if !strings.Contains(pat, "*") {
		return pat == seg
	}
	prefix, suffix, _ := strings.Cut(pat, "*")
	return strings.HasPrefix(seg, prefix) && strings.HasSuffix(seg, suffix) &&
		len(seg) >= len(prefix)+len(suffix)
}
