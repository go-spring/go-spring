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

// Package i18n resolves localized messages: a [MessageSource] turns a key plus
// arguments into a string in the caller's language. It serves user-facing
// business text and the rendering of validation.ValidationErrors, with zero
// third-party dependencies.
//
// The locale travels on the context ([WithLocale] / [LocaleFrom]) so a request
// can carry the client's Accept-Language once and every downstream Message call
// picks it up implicitly, the same way trace context flows.
//
// The bundled [MapSource] holds messages in memory: locale -> key -> template.
// It reads no files; the wiring layer parses properties/yaml/json (or fetches
// them from a config center) and feeds the resulting maps in via
// [MapSource.AddMap] / [MapSource.AddParsed].
package i18n

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// ErrMessageNotFound is returned (wrapped) by a [MessageSource] when a key
// resolves in neither the requested locale nor the fallback locale. Callers that
// want fail-loud behaviour test for it with errors.Is; callers that want a
// graceful fallback ignore the error and use the returned string, which is the
// key itself.
var ErrMessageNotFound = errors.New("i18n: message not found")

type localeKey struct{}

// WithLocale returns a copy of ctx carrying locale (e.g. "zh", "en"). An empty
// locale is stored as-is and later treated as "no locale requested".
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeKey{}, locale)
}

// LocaleFrom returns the locale stored on ctx, or "" when none is set.
func LocaleFrom(ctx context.Context) string {
	if v, ok := ctx.Value(localeKey{}).(string); ok {
		return v
	}
	return ""
}

// MessageSource resolves key into a localized string, interpolating args. The
// locale is taken from ctx (see [LocaleFrom]); an implementation may fall back
// to its own configured fallback locale when the lookup misses.
//
// On a missing key the contract is: return the key unchanged together with an
// error wrapping [ErrMessageNotFound], so a caller may either surface the error
// or use the key as a last-resort label.
type MessageSource interface {
	Message(ctx context.Context, key string, args ...any) (string, error)
}

// MapSource is the bundled, in-memory [MessageSource]: it holds templates keyed
// by locale then message key. It is safe for concurrent read while messages are
// added at wiring time; adding after serving begins is allowed but should be
// guarded by the caller if it races with reads (the internal lock keeps the map
// itself consistent).
//
// Lookup order for Message(ctx, key): the ctx locale, then the fallback locale
// set by [WithFallbackLocale], then the key itself with [ErrMessageNotFound].
// The fallback locale is the safety net for partially translated bundles.
type MapSource struct {
	fallbackLocale string
	mu             sync.RWMutex
	messages       map[string]map[string]string // locale -> key -> template
}

// MapSourceOption customizes a [MapSource] at construction.
type MapSourceOption func(*MapSource)

// WithFallbackLocale sets the locale consulted when a key is absent in the
// requested locale — pick the one your bundle is most complete in. Without it
// there is no fallback layer: a missing key in the request locale resolves to
// the key itself with [ErrMessageNotFound].
func WithFallbackLocale(locale string) MapSourceOption {
	return func(s *MapSource) { s.fallbackLocale = locale }
}

// NewMapSource creates an empty [MapSource].
func NewMapSource(opts ...MapSourceOption) *MapSource {
	s := &MapSource{messages: map[string]map[string]string{}}
	for _, opt := range opts {
		opt(s)
	}
	return s
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

// AddMap registers every key/template pair for one locale.
func (s *MapSource) AddMap(locale string, m map[string]string) *MapSource {
	for k, v := range m {
		s.Add(locale, k, v)
	}
	return s
}

// AddParsed registers messages for one locale from a nested map (the shape a
// yaml/json parser yields, e.g. {"validation":{"email":"..."}}). Nested maps are
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
	if s.fallbackLocale != "" && !strings.EqualFold(locale, s.fallbackLocale) {
		if t, ok := s.lookup(s.fallbackLocale, key); ok {
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

// interpolate replaces positional placeholders {0}, {1}, ... in template with
// the string form of the matching arg. Unmatched placeholders are left intact so
// a template mismatch is visible rather than silently dropped.
func interpolate(template string, args ...any) string {
	if len(args) == 0 || !strings.ContainsRune(template, '{') {
		return template
	}
	var b strings.Builder
	for i := 0; i < len(template); {
		if template[i] == '{' {
			if j := strings.IndexByte(template[i:], '}'); j > 0 {
				token := template[i+1 : i+j]
				if n, err := strconv.Atoi(token); err == nil && n >= 0 && n < len(args) {
					b.WriteString(argString(args[n]))
					i += j + 1
					continue
				}
			}
		}
		b.WriteByte(template[i])
		i++
	}
	return b.String()
}

func argString(a any) string {
	switch v := a.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}
