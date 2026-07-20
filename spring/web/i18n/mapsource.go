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

package i18n

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// MapSource is the bundled, in-memory [MessageSource]: it holds templates keyed
// by locale then message key. It is safe for concurrent read while messages are
// added at wiring time; adding after serving begins is allowed but should be
// guarded by the caller if it races with reads (the internal lock keeps the map
// itself consistent).
//
// Lookup order for Message(ctx, key): the ctx locale, then the default locale,
// then the key itself with [ErrMessageNotFound]. This makes the default locale
// the safety net for partially translated bundles.
type MapSource struct {
	defaultLocale string
	mu            sync.RWMutex
	messages      map[string]map[string]string // locale -> key -> template
}

// NewMapSource creates an empty [MapSource] whose default locale is
// defaultLocale (the fallback consulted when a key is absent in the requested
// locale). An empty defaultLocale disables the fallback layer.
func NewMapSource(defaultLocale string) *MapSource {
	return &MapSource{
		defaultLocale: defaultLocale,
		messages:      map[string]map[string]string{},
	}
}

// Add registers a single template for one key in one locale, overwriting any
// previous value. It returns the receiver for call chaining.
func (s *MapSource) Add(locale, key, template string) *MapSource {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.messages[locale]
	if m == nil {
		m = map[string]string{}
		s.messages[locale] = m
	}
	m[key] = template
	return s
}

// AddMap registers every key/template pair for one locale. It is the shape the
// conf reader produces for a flat properties file.
func (s *MapSource) AddMap(locale string, m map[string]string) *MapSource {
	for k, v := range m {
		s.Add(locale, k, v)
	}
	return s
}

// AddParsed registers messages for one locale from the nested map a conf reader
// yields for yaml/json (e.g. {"validation":{"email":"..."}}). Nested maps are
// flattened with dot-joined keys so {"validation":{"email":"x"}} maps to key
// "validation.email". Both map[string]any (json) and map[any]any (yaml.v2) nests
// are handled; non-string leaves are rendered with fmt.Sprint.
func (s *MapSource) AddParsed(locale string, m map[string]any) *MapSource {
	flat := map[string]string{}
	for k, v := range m {
		flatten(k, v, flat)
	}
	return s.AddMap(locale, flat)
}

func flatten(prefix string, v any, out map[string]string) {
	switch child := v.(type) {
	case map[string]any:
		for k, cv := range child {
			flatten(prefix+"."+k, cv, out)
		}
	case map[any]any:
		for k, cv := range child {
			flatten(prefix+"."+fmt.Sprint(k), cv, out)
		}
	case string:
		out[prefix] = child
	default:
		out[prefix] = fmt.Sprint(child)
	}
}

// Message implements [MessageSource]. See [MapSource] for the lookup order.
func (s *MapSource) Message(ctx context.Context, key string, args ...any) (string, error) {
	locale := LocaleFrom(ctx)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if t, ok := s.lookup(locale, key); ok {
		return interpolate(t, args...), nil
	}
	if s.defaultLocale != "" && !strings.EqualFold(locale, s.defaultLocale) {
		if t, ok := s.lookup(s.defaultLocale, key); ok {
			return interpolate(t, args...), nil
		}
	}
	return key, fmt.Errorf("%w: key=%q locale=%q", ErrMessageNotFound, key, locale)
}

func (s *MapSource) lookup(locale, key string) (string, bool) {
	if locale == "" {
		return "", false
	}
	if m, ok := s.messages[locale]; ok {
		if t, ok := m[key]; ok {
			return t, true
		}
	}
	return "", false
}

// Localizer curries Message to a fixed ctx, producing the func(key, args...)
// string signature that validation.ValidationErrors.Localize expects. A missing
// key yields "" (the [ErrMessageNotFound] error is swallowed) so the caller's
// own default message is used instead of leaking a raw key.
func Localizer(src MessageSource, ctx context.Context) func(key string, args ...any) string {
	return func(key string, args ...any) string {
		s, err := src.Message(ctx, key, args...)
		if err != nil {
			return ""
		}
		return s
	}
}
