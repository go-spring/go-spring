/*
 * Copyright 2024 The Go-Spring Authors.
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

package provider

import (
	"context"
	"os"
	"strings"

	"go-spring.org/spring/conf/reader"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

var providers = map[string]Provider{}

func init() {
	Register("file", ProviderFunc(LoadFile))
}

// Provider reads configuration data from a specific source type (file, nacos,
// etcd, ...). If access to a remote server is needed, the information required
// to access it is usually placed in the source string or passed through
// environment variables.
//
// A source that supports hot-reload installs its watcher or listener inside
// Load, and Close releases whatever Load installed. The two sides are not
// symmetric: Load runs at startup and again on every property refresh, while
// Close runs once when the application shuts down. An implementation must
// tolerate every interleaving — Close on a provider that never loaded is a
// no-op, and Load after Close re-arms, because one process can run several
// application instances in sequence (e.g. repeated gs.RunTest runs).
type Provider interface {
	// Load returns the source content as a flattened map[string]string.
	// When optional is true and the source does not exist, it returns (nil, nil).
	// The context carries the refresh's cancellation and trace, so a remote
	// source can bound its network calls with it.
	Load(ctx context.Context, optional bool, source string) (map[string]string, error)

	// Close stops everything Load installed (watchers, listeners, clients).
	// It must be safe to call repeatedly, reports nothing and takes no context:
	// closing tears down local resources synchronously, so there is nothing to
	// cancel, no trace to carry, and no failure a caller could act on — a
	// provider that cannot close a resource logs it itself.
	Close()
}

// ProviderFunc adapts a plain read function to Provider, for a source that
// holds no resource to release.
type ProviderFunc func(ctx context.Context, optional bool, source string) (map[string]string, error)

// Load implements Provider.
func (f ProviderFunc) Load(ctx context.Context, optional bool, source string) (map[string]string, error) {
	return f(ctx, optional, source)
}

// Close implements Provider: a plain function installs nothing, so there is
// nothing to stop.
func (ProviderFunc) Close() {}

// Register registers a Provider for a specific configuration source type.
// Must be called in init functions only.
func Register(name string, p Provider) {
	if name == "" {
		panic("provider name cannot be empty")
	}
	if p == nil {
		panic("provider " + name + " cannot be nil")
	}
	if _, ok := providers[name]; ok {
		panic("provider " + name + " already exists")
	}
	providers[name] = p
}

// Load loads a configuration source and returns its content as a flattened map[string]string.
// The context carries the refresh's cancellation and trace; a provider talking
// to a remote server should bound its network calls with it.
// The source string format is:
//
//	[optional:]<provider>:<path>
//	<path> (defaults to file provider)
//
// Examples:
//   - "file:./config.yaml"                    // file provider, required
//   - "optional:file:./config.yaml"           // file provider, optional
//   - "./config.yaml"                         // shorthand for file:./config.yaml
//   - "etcd:localhost:2379/config"            // custom provider
//   - "optional:etcd:localhost:2379/config"   // custom provider, optional
//
// When optional is true and the source does not exist, Load returns (nil, nil).
func Load(ctx context.Context, source string) (map[string]string, error) {
	// For example, a spring.config.import value of optional:file:./myconfig.properties
	// allows your application to start, even if the myconfig.properties file is missing.

	var (
		config   = source
		provider = "file"
		optional bool
	)

	// Parse the source string in format [optional:]<provider>:<path> or just <path>.
	if s, ok := strings.CutPrefix(source, "optional:"); ok {
		optional = true
		source = s
	}
	if p, s, ok := strings.Cut(source, ":"); ok {
		provider = p
		source = s
	}

	p, ok := providers[provider]
	if !ok {
		err := errutil.Explain(nil, "unsupported provider type %s", provider)
		return nil, errutil.Explain(err, "conf: read config %q error", config)
	}
	m, err := p.Load(ctx, optional, source)
	if err != nil {
		return nil, errutil.Explain(err, "conf: read config %q error", config)
	}
	return m, nil
}

// CloseAll stops every registered provider, in no defined order. Providers are
// independent and must not rely on one another having been closed. The registry
// itself is left intact: registration happens in init, but Close runs once per
// application instance, and a later instance must still find its providers.
func CloseAll() {
	for _, p := range providers {
		p.Close()
	}
}

// LoadFile loads a configuration file and returns its content as a flattened map[string]string.
// If the file does not exist and optional is true, it returns nil without error.
func LoadFile(ctx context.Context, optional bool, source string) (map[string]string, error) {
	m, err := reader.ReadFile(source)
	if err != nil {
		if os.IsNotExist(err) && optional {
			return nil, nil
		}
		return nil, err
	}
	return flatten.Flatten(m), nil
}
