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

// Package StarterGovernance provides dynamic-refresh source adapters for the
// governance center (cloud/governance) — the self-built refresh chain, where
// governance rules flow in through the governance.Source contract instead of
// the ${govern} gs.Dync binding.
//
// Importing this starter is inert until it is configured; the default
// ${govern} Dync path stays exactly as it is. When a source IS configured,
// its bean is injected onto the governance center (priority: an explicit
// governance.SetSource wins, then this bean, then the Dync default), and rule
// updates refresh governance only — they never trigger an app-wide property
// re-bind.
//
// Available sources:
//
//   - "file" (source_file.go) — one standalone rules file, watched with
//     fsnotify. Keys are the same govern.* keys an app.properties entry would
//     use; format by extension (json/properties/yaml/toml). Configure with:
//
//     govern.source.file.path=/etc/app/govern.yaml
//
//   - "http" (source_http.go) — polls a governance console / rules API on a
//     fixed interval and converges on whatever document it serves. Configure
//     with:
//
//     govern.source.http.url=https://console.example.com/rules/app.yaml
//     govern.source.http.interval=10s
//     govern.source.http.format=yaml
//     govern.source.http.headers.authorization=Bearer xxx
//
// Exactly one source may be active per process (the center holds one). Remote
// config-center adapters (nacos/etcd direct listeners) live in their own
// config starters, next to the clients they reuse.
package StarterGovernance

import (
	"time"

	"go-spring.org/cloud/governance"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

var starterTag = log.RegisterAppTag("governance", "")

// fileSourceConfig is the "file" source's own configuration, bound from
// ${govern.source.file.*} (the source config lives under govern.*, next to
// the rules it feeds — one namespace, two roles: govern.* is the rules,
// govern.source.* is where the rules come from).
type fileSourceConfig struct {
	// Path is the rules file to watch. Required.
	Path string `value:"${path}" expr:"$ != ''"`
}

// httpSourceConfig is the "http" source's configuration, bound from
// ${govern.source.http.*}.
type httpSourceConfig struct {
	// URL is the console/rules endpoint to poll. Required.
	URL string `value:"${url}" expr:"$ != ''"`

	// Interval is the poll period (default 5s).
	Interval time.Duration `value:"${interval:=5s}"`

	// Format overrides document-format detection ("yaml", "json",
	// "properties", "toml"); by default it is inferred from the URL path.
	Format string `value:"${format:=}"`

	// Headers are sent with every request (e.g. authorization=Bearer xxx).
	Headers map[string]string `value:"${headers:=}"`
}

func init() {
	// One source per process (the center holds exactly one active source), so
	// this is a plain conditional bean, not a Group. OnProperty is a prefix
	// check: any govern.source.file.* key arms the module.
	gs.Module(gs.OnProperty("govern.source.file"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c fileSourceConfig
		if err := conf.Bind(p, &c, "${govern.source.file:=}"); err != nil {
			return err
		}

		// Exported as governance.Source so the center's Src field (autowire:"?")
		// finds it. Without the Export the bean is invisible to interface
		// injection and governance silently runs on ${govern} again — the
		// most likely misconfiguration, called out in the README.
		r.Provide(func() (*FileSource, error) { return NewFileSource(c.Path) }).
			Init((*FileSource).Init).Destroy((*FileSource).Close).
			Export(gs.As[governance.Source]()).Caller(1)
		return nil
	})

	// "http": poll a governance console. Same registration shape as "file" —
	// each adapter is a conditional singleton exported as governance.Source.
	gs.Module(gs.OnProperty("govern.source.http"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c httpSourceConfig
		if err := conf.Bind(p, &c, "${govern.source.http:=}"); err != nil {
			return err
		}
		r.Provide(func() (*HTTPSource, error) { return NewHTTPSource(c.URL, c.Interval, c.Format, c.Headers) }).
			Init((*HTTPSource).Init).Destroy((*HTTPSource).Close).
			Export(gs.As[governance.Source]()).Caller(1)
		return nil
	})
}
