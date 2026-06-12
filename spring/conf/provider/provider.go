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
	"os"
	"strings"

	"go-spring.org/spring/conf/reader"
	"github.com/go-spring/stdlib/errutil"
	"github.com/go-spring/stdlib/flatten"
)

var providers = map[string]Provider{}

func init() {
	Register("file", LoadFile)
}

// Provider defines a function type that provides configuration data from a specific source.
//
// If access to a remote server is needed, the information required to access it is usually
// placed in the source string or passed through environment variables.
//
// To support dynamic refresh, a version number field can be added and a regular Bean
// can be registered to listen for changes. When the version changes, the configuration
// can be updated.
type Provider func(optional bool, source string) (map[string]string, error)

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
//
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
func Load(source string) (map[string]string, error) {
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
	m, err := p(optional, source)
	if err != nil {
		return nil, errutil.Explain(err, "conf: read config %q error", config)
	}
	return m, nil
}

// LoadFile loads a configuration file and returns its content as a flattened map[string]string.
// If the file does not exist and optional is true, it returns nil without error.
func LoadFile(optional bool, source string) (map[string]string, error) {
	m, err := reader.ReadFile(source)
	if err != nil {
		if os.IsNotExist(err) && optional {
			return nil, nil
		}
		return nil, err
	}
	return flatten.Flatten(m), nil
}
