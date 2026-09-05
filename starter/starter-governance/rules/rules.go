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

// Package rules is the shared parse glue between spring/conf and the
// container-free governance core: it turns one governance rules document (a
// file's bytes, an HTTP body, a config-center value — the same bytes any
// [governance.Source] backend delivers) into a governance.Config.
//
// It deliberately lives as a SUBPACKAGE of starter-governance, not inside
// cloud/governance: parsing needs spring/conf's value-tag driver, and the
// cloud module must not depend on the spring module at all. Being a subpackage
// also means importing it does NOT run the starter's root-package wiring init
// (Go only initializes packages actually imported), so the nacos/etcd source
// adapters in other starters can share it without pulling in the default
// ${govern} wiring as a side effect.
package rules

import (
	"strings"

	"go-spring.org/cloud/governance"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// Parse turns one governance rules document into a governance.Config. Every
// Source adapter across starters parses through this one function, so every
// backend accepts identical documents with identical semantics: a rules file
// that works with a local file source works unchanged as a nacos dataId, an
// etcd key value, or a console payload.
//
// Binding goes through the same conf value-tag machinery the ${govern} Dync
// binding uses (prefix "govern"), so documents use the same keys an
// app.properties entry would. format names the document format ("yaml",
// "json", "properties", "toml"); when empty it is inferred from name's
// extension, defaulting to properties.
//
// A document that parses but carries NO govern.* keys is an error, not "no
// governance": properties syntax almost never hard-fails, so a truncated or
// emptied document would otherwise silently disarm the whole center. Turning
// governance off is `govern.enabled=false` — a key that IS present.
func Parse(name string, data []byte, format string) (governance.Config, error) {
	if format == "" {
		format = formatOf(name)
	}
	parsed, err := reader.Read(format, data)
	if err != nil {
		return governance.Config{}, errutil.Explain(err, "governance source: parse %s failed", name)
	}
	m := flatten.Flatten(parsed)
	if !hasGovernKey(m) {
		return governance.Config{}, errutil.Explain(nil, "governance source: %s contains no govern.* keys (empty or truncated?)", name)
	}

	var cfg governance.Config
	if err = conf.Bind(flatten.NewPropertiesStorage(flatten.NewProperties(m)), &cfg, "${govern:=}"); err != nil {
		return governance.Config{}, errutil.Explain(err, "governance source: bind %s failed", name)
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
		if strings.HasPrefix(k, "govern.") {
			return true
		}
	}
	return false
}
