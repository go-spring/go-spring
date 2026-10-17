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
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

// This package registers the built-in SOURCE adapters, and nothing else. The
// process's composition root — the bean a client or server starter injects — is
// [go-spring.org/cloud/runtime.Runtime], registered by the runtime root. Keeping
// the two apart is what preserves the plane boundary: governance owns the
// document and its transport, while the runtime owns the authorities and hands
// them out.
//
// Governance's configuration is its OWN system — a rules file watched by the
// file source, a remote console via the http source, or any other Source — and
// deliberately NOT bound through gs properties: a governance rule change
// refreshes governance only, never re-binds the whole app.
func init() {
	// The two built-in source adapters, registered by the package that owns
	// them: linking cloud/runtime/controlplane/governance is enough to get the file and http sources,
	// with no separate starter to import. One source per process (the center
	// holds exactly one active source), so each is a plain conditional bean, not
	// a Group; OnProperty is a prefix check, so any
	// spring.governance.source.<kind>.* key arms its module.
	//
	// Both are exported as [Source], which is what the center's constructor
	// collects. Without the Export the bean is invisible to interface injection
	// and startup fails on that required parameter — the most likely
	// misconfiguration, called out in the README.
	gs.Module(gs.OnProperty("spring.governance.source.file"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c fileSourceConfig
		if err := conf.Bind(p, &c, "${spring.governance.source.file:=}"); err != nil {
			return err
		}
		r.Provide(func() (*FileSource, error) { return NewFileSource(c.Path) }).
			Init((*FileSource).Init).Destroy((*FileSource).Close).
			Export(gs.As[Source]()).Caller(1)
		return nil
	})

	// "http": poll a governance console. Same registration shape as "file".
	gs.Module(gs.OnProperty("spring.governance.source.http"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c httpSourceConfig
		if err := conf.Bind(p, &c, "${spring.governance.source.http:=}"); err != nil {
			return err
		}
		r.Provide(func() (*HTTPSource, error) { return NewHTTPSource(c.URL, c.Interval, c.Format, c.Headers) }).
			Init((*HTTPSource).Init).Destroy((*HTTPSource).Close).
			Export(gs.As[Source]()).Caller(1)
		return nil
	})
}

var starterTag = log.RegisterAppTag("governance", "")
