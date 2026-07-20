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

// RequireField returns a fail-fast error when a required configuration value is
// empty, using the standard "<component>: <field> is required" wording that the
// starters were each spelling out by hand. component is the starter's short
// name (e.g. "redis", "mail"); field is the human-readable property name (e.g.
// "host", "addr"). It returns nil when the value is present.
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
// is empty, for the common "addr OR service-name" rule that go-spring's `expr:`
// tag cannot express (it validates one field at a time). The message reads
// "<component>: one of <a> or <b> is required". It returns nil as soon as any
// value is present.
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

// Field is a named configuration value passed to RequireAny.
type Field struct {
	Name  string
	Value string
}
