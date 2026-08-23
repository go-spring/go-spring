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

package errutil

import "strings"

// Field is a named string value passed to RequireAny. Name is the
// human-readable property name used in the error message (e.g. "addr",
// "service-name"); Value is the field's current value.
type Field struct {
	Name  string
	Value string
}

// RequireField returns a fail-fast error when a required value is empty, using
// the standard "<component>: <field> is required" wording that the starters
// were each spelling out by hand. component is the component's short name
// (e.g. "redis", "mail"); field is the human-readable property name (e.g.
// "host", "addr"). value is tested with strings.TrimSpace, so a whitespace-only
// string counts as empty. It returns nil when the value is present.
//
//	if err := errutil.RequireField("mail", "host", cfg.Host); err != nil {
//	    return nil, err
//	}
func RequireField(component, field, value string) error {
	if strings.TrimSpace(value) == "" {
		return Explain(nil, "%s: %s is required", component, field)
	}
	return nil
}

// RequireAny returns a fail-fast error when every one of the alternative fields
// is empty, for the common "addr OR service-name" rule that spring/conf's
// `expr:` tag cannot express (it validates one field at a time). It scans the
// fields in order and returns nil as soon as the first non-empty (after
// TrimSpace) value is found; only when all are empty does it build an error
// reading "<component>: one of <a> or <b> [or <c>...] is required", where the
// field names are joined with " or " in the order given.
//
// Callers should pass at least two fields: a single-field "required" check is
// RequireField's job, and a one-field RequireAny would read "one of <a> is
// required" — grammatical nonsense. A zero-field call produces an empty name
// list and is a caller bug.
//
//	if err := errutil.RequireAny("http-client",
//	    errutil.Field{Name: "addr", Value: cfg.Addr},
//	    errutil.Field{Name: "service-name", Value: cfg.ServiceName},
//	); err != nil {
//	    return nil, err
//	}
func RequireAny(component string, fields ...Field) error {
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		if strings.TrimSpace(f.Value) != "" {
			return nil
		}
		names = append(names, f.Name)
	}
	return Explain(nil, "%s: one of %s is required", component, strings.Join(names, " or "))
}
