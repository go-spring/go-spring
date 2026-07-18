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

// Package StarterResilience registers sentinel-golang as the recommended
// resilience driver for the framework defined in
// [go-spring.org/stdlib/resilience].
//
// It is enabled purely by a blank import:
//
//	import _ "go-spring.org/starter-resilience"
//
// After that, any adapter (starter-oauth2-client, ...) that reads
// ${...resilience.driver} can select "sentinel" with no code change — this is
// the whole point of the abstraction/driver split. The bundled zero-dependency
// "default" driver stays available for tests and lightweight use; this module
// exists so production traffic can lean on sentinel-golang's adaptive flow
// control and circuit breaking instead.
package StarterResilience

import (
	sentinel "github.com/alibaba/sentinel-golang/api"

	"go-spring.org/stdlib/resilience"
)

func init() {
	// Initialise sentinel once at import time. Registration then always succeeds
	// so a misconfigured environment fails loudly here rather than on first use.
	if err := sentinel.InitDefault(); err != nil {
		panic("starter-resilience: sentinel init failed: " + err.Error())
	}
	resilience.RegisterDriver("sentinel", sentinelDriver{})
}

type sentinelDriver struct{}

func (sentinelDriver) NewExecutor(p resilience.Policy) (resilience.Executor, error) {
	return newSentinelExecutor(p)
}
