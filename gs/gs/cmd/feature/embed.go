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

package feature

import (
	_ "embed"
	"sync"
)

// embeddedManifest is the feature list compiled into the gs binary. It MUST be
// compiled in (not fetched from the cloned layout at runtime), because cobra
// registers flags before argv is parsed and the feature set defines those
// flags. Adding/removing a feature is therefore a gs release: edit this JSON
// and rebuild. Keep it in sync with the layout superset it prunes.
//
//go:embed features.json
var embeddedManifest []byte

var (
	embeddedOnce sync.Once
	embedded     *Manifest
	embeddedErr  error
)

// Embedded returns the manifest compiled into gs. It is the source of truth for
// which feature flags `gs init` (and `gs add`) expose.
func Embedded() (*Manifest, error) {
	embeddedOnce.Do(func() {
		embedded, embeddedErr = parse(embeddedManifest, "embedded features.json")
	})
	return embedded, embeddedErr
}
