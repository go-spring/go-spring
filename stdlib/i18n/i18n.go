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

// Package i18n defines a zero-dependency abstraction for localized messages: a
// [MessageSource] resolves a key plus arguments into a string in the caller's
// language. It backs two things in the framework — user-facing business text
// and the rendering of validation.ValidationErrors — without pulling any
// third-party i18n library into the foundation layer.
//
// The locale travels on the context ([WithLocale] / [LocaleFrom]) so a request
// can carry the client's Accept-Language once and every downstream Message call
// picks it up implicitly, the same way trace context flows.
//
// The bundled [MapSource] holds messages in memory: locale -> key -> template.
// Because stdlib may not depend on spring/conf/reader (that would reverse the
// dependency direction), MapSource does not read files itself; callers parse
// their properties/yaml/json with the conf reader and feed the resulting maps in
// via [MapSource.AddMap] / [MapSource.AddParsed]. That keeps this package
// zero-dependency while still "reusing the reader" at the wiring layer, and the
// same seam accepts maps fetched from a remote config-center provider.
package i18n

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrMessageNotFound is returned (wrapped) by a [MessageSource] when a key
// resolves in neither the requested locale nor the default locale. Callers that
// want fail-loud behaviour test for it with errors.Is; callers that want a
// graceful fallback ignore the error and use the returned string, which is the
// key itself.
var ErrMessageNotFound = errors.New("i18n: message not found")

// MessageSource resolves key into a localized string, interpolating args. The
// locale is taken from ctx (see [LocaleFrom]); an implementation falls back to
// its configured default locale when the context carries none.
//
// On a missing key the contract is: return the key unchanged together with an
// error wrapping [ErrMessageNotFound], so a caller may either surface the error
// or use the key as a last-resort label.
type MessageSource interface {
	Message(ctx context.Context, key string, args ...any) (string, error)
}

type localeKey struct{}

// WithLocale returns a copy of ctx carrying locale (e.g. "zh", "en"). An empty
// locale is stored as-is and later treated as "use the default".
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
