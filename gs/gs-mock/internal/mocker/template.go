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

package main

import (
	"text/template"
)

var tmplMock = template.Must(template.New("").Parse(`
/******************************** {{.mockerName}} ***********************************/

// {{.mockerName}} provides a configurable mock for the target function.
type {{.mockerName}}{{.typeParams}} struct {
	fnHandle func({{.req}}) {{.resp}}
	fnWhen   func({{.req}}) bool
	fnReturn func() {{.resp}}
}

// Handle sets a custom handler function for intercepted calls.
func (m *{{.mockerName}}{{.typeArgs}}) Handle(fn func({{.req}}) {{.resp}}) {
	m.fnHandle = fn
}

// When sets a predicate function that determines whether the mock applies.
func (m *{{.mockerName}}{{.typeArgs}}) When(fn func({{.req}}) bool) *{{.mockerName}}{{.typeArgs}} {
	m.fnWhen = fn
	return m
}

// Return sets a function that produces return values when the mock is matched.
func (m *{{.mockerName}}{{.typeArgs}}) Return(fn func() {{.resp}}) {
	if m.fnWhen == nil {
		m.fnWhen = func({{.req}}) bool { return true }
	}
	m.fnReturn = fn
}

// ReturnValue is a convenience wrapper around Return that uses fixed values.
func (m *{{.mockerName}}{{.typeArgs}}) ReturnValue({{.respParams}}) {
	m.Return(func() {{.resp}} { {{if .respVars}} return {{.respVars}} {{end}} })
}

// ReturnDefault configures the mock to return zero values for all return types.
func (m *{{.mockerName}}{{.typeArgs}}) ReturnDefault() {
	m.Return(func() ({{.respParams}}) { {{if .respVars}} return {{.respVars}} {{end}} })
}

// {{.invokerName}} implements Invoker for {{.mockerName}}.
type {{.invokerName}}{{.typeParams}} struct {
	*{{.mockerName}}{{.typeArgs}}
}

// Invoke dispatches the call to the configured handler or return function.
func (m *{{.invokerName}}{{.typeArgs}}) Invoke(params []any) ([]any, bool) {
	if m.fnHandle != nil {
		{{if .respVars}} {{.respVars}} := {{end}} m.fnHandle({{.invokerArgs}})
		return []any{ {{if .respVars}} {{.respVars}} {{end}} }, true
	}
	if m.fnWhen != nil {
		if ok := m.fnWhen({{.invokerArgs}}); ok {
			{{if .respVars}} {{.respVars}} := {{end}} m.fnReturn()
			return []any{ {{if .respVars}} {{.respVars}} {{end}} }, true
		}
	}
	return nil, false
}

// {{.funcMockName}} creates a new {{.mockerName}} and registers it with the Manager.
func {{.funcMockName}}{{.typeParams}}(f func({{.funcReq}}) {{.resp}}, r *Manager) *{{.mockerName}}{{.typeArgs}} {
	PatchOnce(f)
	m := &{{.mockerName}}{{.typeArgs}}{}
	i := &{{.invokerName}}{{.typeArgs}}{ {{.mockerName}}: m}
	r.addInvoker(nil, f, i)
	return m
}

// {{.methodMockName}} creates a new {{.mockerName}} for mocking a method on a receiver.
func {{.methodMockName}}{{.typeParams}}(receiver any, f func({{.funcReq}}) {{.resp}}, r *Manager) *{{.mockerName}}{{.typeArgs}} {
	m := &{{.mockerName}}{{.typeArgs}}{}
	i := &{{.invokerName}}{{.typeArgs}}{ {{.mockerName}}: m}
	r.addInvoker(receiver, f, i)
	return m
}
`))
