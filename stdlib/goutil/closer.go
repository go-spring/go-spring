/*
 * Copyright 2024 The Go-Spring Authors.
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

package goutil

import "io"

// CloserFunc adapts a plain function to io.Closer, so a client starter's Driver
// can hand back the teardown for whatever it built (e.g. stopping a discovery
// resolver watch) without keeping a client->resolver side-channel registry. A
// nil CloserFunc is a valid no-op Close.
type CloserFunc func() error

// Close implements io.Closer. It is a no-op for a nil CloserFunc.
func (f CloserFunc) Close() error {
	if f == nil {
		return nil
	}
	return f()
}

// NopCloser returns an io.Closer whose Close is a no-op, for a component that
// needs no extra teardown (e.g. one that never created a background watcher).
func NopCloser() io.Closer { return CloserFunc(nil) }
