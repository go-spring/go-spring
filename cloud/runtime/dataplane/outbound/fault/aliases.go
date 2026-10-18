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

package fault

import kfault "go-spring.org/cloud/runtime/dataplane/kernel/fault"

// Re-exported neutral fault vocabulary. A client that only drives the outbound
// authority can name a Target, a Decision and a rule document without also
// importing the kernel package; the aliases make the two spellings identical.
type (
	Config        = kfault.Config
	Rule          = kfault.Rule
	Target        = kfault.Target
	Decision      = kfault.Decision
	ActionMatcher = kfault.ActionMatcher
	InjectedError = kfault.InjectedError
)

// ErrInjected is the sentinel an injected failure wraps.
var ErrInjected = kfault.ErrInjected

// IsInjected reports whether err came from an injected fault.
func IsInjected(err error) bool { return kfault.IsInjected(err) }
