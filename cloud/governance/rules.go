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
	"strings"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// Parse is the one place in cloud/ that reaches for spring: a document is bound
// through spring/conf's value-tag machinery, so it uses the same keys an
// app.properties entry would. The coupling is deliberate — the document format
// is the framework's, shared by every governance source, and a per-backend
// parser would let the backends drift apart. It lives in the root package so
// the family has one obvious parse entry point.
//
// Parse turns one governance rules document into a Config. Every
// Source adapter across starters parses through this one function, so every
// backend accepts identical documents with identical semantics: a rules file
// that works with a local file source works unchanged as a nacos dataId, an
// etcd key value, or a console payload.
//
// Binding goes through the same conf value-tag machinery (prefix "spring.governance"), so
// documents use the same keys an app.properties entry would. format names the
// document format ("yaml",
// "json", "properties", "toml"); when empty it is inferred from name's
// extension, defaulting to properties.
//
// A document that parses but carries NO spring.governance.* keys is an error, not "no
// governance": properties syntax almost never hard-fails, so a truncated or
// emptied document would otherwise silently disarm the whole center. Turning
// governance off is `spring.governance.enabled=false` — a key that IS present.
func Parse(name string, data []byte, format string) (Config, error) {
	if format == "" {
		format = formatOf(name)
	}
	parsed, err := reader.Read(format, data)
	if err != nil {
		return Config{}, errutil.Explain(err, "governance source: parse %s failed", name)
	}
	m := flatten.Flatten(parsed)
	if !hasGovernKey(m) {
		return Config{}, errutil.Explain(nil, "governance source: %s contains no spring.governance.* keys (empty or truncated?)", name)
	}

	var cfg Config
	if err = conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(m)), &cfg, "${spring.governance:=}"); err != nil {
		return Config{}, errutil.Explain(err, "governance source: bind %s failed", name)
	}
	return cfg, nil
}

// formatOf infers the document format from a name (path, dataId, URL) by
// extension, defaulting to properties — the most common config-center format.
func formatOf(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i:] // reader.Read accepts a dotted extension
	}
	return ".properties"
}

// hasGovernKey reports whether the flattened document carries at least one
// key under the govern namespace.
func hasGovernKey(m map[string]string) bool {
	for k := range m {
		if strings.HasPrefix(k, "spring.governance.") {
			return true
		}
	}
	return false
}
