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

package StarterCasbin

// Config holds the configuration parameters for a Casbin enforcer.
type Config struct {
	Model    string `value:"${model}"`          // Path to the model file (model.conf)
	Policy   string `value:"${policy:=}"`       // Path to the policy file (policy.csv); mutually exclusive with Adapter
	AutoSave bool   `value:"${autoSave:=true}"` // Whether policy mutations are persisted back to the storage

	// Adapter names the persist.Adapter bean the application provides (under
	// this bean name) that backs this enforcer. When set, the enforcer
	// loads/saves policies through that adapter (DB, file, ...) instead of the
	// Policy file, and the named bean is injected into it. Empty means use the
	// file adapter backed by Policy.
	Adapter string `value:"${adapter:=}"`

	// Watcher names the persist.Watcher bean the application provides (under
	// this bean name). When set, the enforcer reloads its policy whenever the
	// watcher signals a change from another instance, enabling hot reload and
	// multi-instance synchronization. Empty means no watcher.
	Watcher string `value:"${watcher:=}"`
}
