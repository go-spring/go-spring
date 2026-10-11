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
	"strings"

	"go-spring.org/log"
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
// emptied document would otherwise silently disarm the whole center. To turn
// governance off, publish a document that configures nothing — empty Client and
// Server blocks, no rules and a zero Default — but it must still carry at least
// one spring.governance.* key, e.g. `spring.governance.client.default.attempt-timeout=1s`.
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
	warnLegacyRetryKeys(name, m)

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

// legacyRetrySuffixes are the retry keys that used to bind directly on a
// client policy. Retry now lives in a `retry` node (chain.RetryPolicy), so these
// spellings no longer bind anywhere — a document that still uses them would
// SILENTLY stop retrying, which is worse than a loud error. Parse warns instead
// of dropping them quietly.
var legacyRetrySuffixes = []string{
	".max-retries", ".retry-budget", ".initial-interval",
	".multiplier", ".max-interval", ".randomization-factor",
}

// warnLegacyRetryKeys logs, once per parse, every governance key that looks like
// a pre-node retry field (a legacy retry suffix NOT under a `.retry.` segment),
// so an operator whose document silently lost its retry sees why. It does not
// fail the parse: the rest of the document is still valid, and a failing parse
// would disarm the whole center — the opposite of what a diagnostic should do.
func warnLegacyRetryKeys(name string, m map[string]string) {
	var dead []string
	for k := range m {
		if !strings.HasPrefix(k, "spring.governance.") || strings.Contains(k, ".retry.") {
			continue
		}
		for _, suffix := range legacyRetrySuffixes {
			if strings.HasSuffix(k, suffix) {
				dead = append(dead, k)
				break
			}
		}
	}
	if len(dead) == 0 {
		return
	}
	log.Warn(context.Background(), log.TagAppDef,
		"governance source: retry keys outside a `retry` node are ignored — retry is declared "+
			"per call, and a center configures it under `<client|server>....retry.*` "+
			"(e.g. client.default.retry.max-retries)",
		log.String("source", name), log.String("keys", strings.Join(dead, ",")))
}
