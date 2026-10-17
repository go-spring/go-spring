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

// Package StarterGovernanceSentinel contributes sentinel-golang as the recommended
// resilience backend for the framework defined in
// [go-spring.org/cloud/resilience].
//
// Contribution is a blank import:
//
//	import _ "go-spring.org/starter-governance-sentinel"
//
// which registers the backend as a bean named "sentinel". Selecting it is then a
// one-line change to the static config — spring.resilience.driver=sentinel — with
// no code change and no per-starter key: the resilience manager injects that
// named bean once at startup and every client picks it up through the same seam.
// This is the whole point of the abstraction/driver split. The bundled
// zero-dependency "default" driver stays available for tests and lightweight use;
// this module exists so production traffic can lean on sentinel-golang's adaptive
// flow control and circuit breaking instead.
package StarterGovernanceSentinel

import (
	"context"
	"go-spring.org/cloud/chain"

	sentinel "github.com/alibaba/sentinel-golang/api"

	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
)

func init() {
	// Initialise sentinel once at import time. Contribution then always succeeds
	// so a misconfigured environment fails loudly here rather than on first use.
	if err := sentinel.InitDefault(); err != nil {
		panic("starter-governance-sentinel: sentinel init failed: " + err.Error())
	}
	// Contribute the backend as a bean NAMED [SentinelName]: the resilience
	// manager injects the [resilience.Driver] bean whose name
	// ${spring.resilience.driver} selects, so this name is what makes the backend
	// selectable. The Export is load-bearing — gs indexes beans by their exact
	// type, so without it the concrete *sentinelDriver would be invisible to that
	// lookup.
	gs.Provide(func() *sentinelDriver { return &sentinelDriver{} }).
		Name(SentinelName).
		Export(gs.As[resilience.Driver]()).
		Caller(1)
	log.Info(context.Background(), log.TagAppDef, "registered sentinel resilience driver")
}

// SentinelName is the bean name this backend answers to, and the value
// ${spring.resilience.driver} takes to select it.
const SentinelName = "sentinel"

type sentinelDriver struct{}

// NewSentinelDriver returns the sentinel-backed [resilience.Driver] as a value,
// for callers that build executors outside the container (examples, tests, a
// program that manages its own wiring). A process using gs does not need it:
// the same backend is contributed as the bean named [SentinelName].
func NewSentinelDriver() resilience.Driver { return sentinelDriver{} }

func (sentinelDriver) NewClientExecutor(service string, p resilience.ClientPolicy) (chain.Executor, error) {
	return newSentinelExecutor(service, p)
}
