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

package governance

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"time"

	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/randutil"
)

// HTTPSource is a Source that PULLS rules from an HTTP endpoint on
// a fixed interval — the governance-console / rules-API pattern (a lightweight
// sibling of Spring Cloud's config-server pull or xDS polling): a control
// plane publishes the current rules document at one URL, and every process
// converges on it without a config center or a file mount.
//
// The body parses through the shared [parseGovernanceDoc] core, so the
// document is byte-compatible with the file source's rules file. Format is
// inferred from the URL path's extension, or set explicitly. A failed fetch,
// a non-200 status, or a bad document keeps the last good snapshot and logs —
// the console being briefly down must not disarm live governance.
type HTTPSource struct {
	url      string
	format   string
	interval time.Duration
	client   *http.Client
	headers  map[string]string // e.g. an Authorization header for the console

	// PushSource is embedded so this transport inherits the [Source] read and
	// subscribe pair (the polling loop feeds it through poll). Embedding also
	// promotes Push — the INTERNAL commit path that skips poll's change-dedup
	// guard — so callers must never call Push directly.
	*PushSource

	// cancel is set by Start and called by Close, with no lock: gs runs Init
	// synchronously while wiring and Destroy exactly once at shutdown, so Start
	// runs at most once and Close at most once, in that order (the polling
	// goroutine reads only construction-fixed fields). Close tolerates a source
	// that was never started — a caller may build one, read its snapshot and drop
	// it without polling.
	cancel context.CancelFunc
}

// NewHTTPSource loads url once and returns an HTTPSource holding that
// snapshot. interval is the poll period; headers are sent with every request.
// The polling loop starts with [HTTPSource.Start] (the gs bean Init hook).
func NewHTTPSource(url string, interval time.Duration, format string, headers map[string]string) (*HTTPSource, error) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	src := &HTTPSource{
		url:     url,
		format:  format,
		client:  &http.Client{Timeout: interval},
		headers: headers,
	}
	cfg, err := src.fetch(context.Background())
	if err != nil {
		return nil, err
	}
	src.PushSource = NewPushSource(cfg)
	src.interval = interval
	return src, nil
}

// Start begins the polling loop. It is the gs bean Init hook, called once, so
// it is single-shot.
func (s *HTTPSource) Start() error {
	// The source's own identity rides on its loop context: the lines below name
	// the URL without repeating it.
	ctx, cancel := context.WithCancel(log.RootFields(log.String("url", s.url)))
	s.cancel = cancel

	go func() {
		tk := time.NewTicker(s.interval)
		defer tk.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
				s.poll(ctx)
			}
		}
	}()
	return nil
}

// Init is the gs lifecycle hook: start polling.
func (s *HTTPSource) Init() error { return s.Start() }

// Close stops the polling loop. Implements the optional-close contract the
// governance center probes for on Destroy.
func (s *HTTPSource) Close() error {
	if s.cancel == nil { // never started — nothing to stop
		return nil
	}
	s.cancel()
	return nil
}

// poll fetches once and, on a good fetch that actually changed the rules,
// pushes it.
func (s *HTTPSource) poll(ctx context.Context) {
	cfg, err := s.fetch(ctx)
	if err != nil {
		log.Error(ctx, starterTag, err, "governance http source: poll failed (keeping last good config)")
		return
	}
	if reflect.DeepEqual(s.Snapshot(), cfg) {
		return
	}
	// A changed document starts a path here and continues into the center that
	// applies it: the change mints the trace_id both sides log against.
	s.Push(log.RootFields(
		log.String("trace_id", randutil.Hex(16)),
		log.String("url", s.url)), cfg)
}

// fetch performs one GET and parses the body through the shared core.
func (s *HTTPSource) fetch(ctx context.Context) (Config, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return Config{}, errutil.Explain(err, "governance http source: build request for %s failed", s.url)
	}
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return Config{}, errutil.Explain(err, "governance http source: fetch %s failed", s.url)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Config{}, errutil.Explain(nil, "governance http source: %s returned %s", s.url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Config{}, errutil.Explain(err, "governance http source: read %s body failed", s.url)
	}
	return Parse(s.url, body, s.format)
}

// httpSourceConfig is the "http" source's configuration, bound from
// ${spring.governance.source.http.*}.
type httpSourceConfig struct {
	// URL is the console/rules endpoint to poll. Required.
	URL string `value:"${url}" expr:"$ != ''"`

	// Interval is the poll period (default 5s).
	Interval time.Duration `value:"${interval:=5s}" expr:"int($) > 0"`

	// Format overrides document-format detection ("yaml", "json",
	// "properties", "toml"); by default it is inferred from the URL path.
	Format string `value:"${format:=}"`

	// Headers are sent with every request (e.g. authorization=Bearer xxx).
	Headers map[string]string `value:"${headers:=}"`
}
