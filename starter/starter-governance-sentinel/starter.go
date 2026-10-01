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
// one-line change to the governance document — spring.governance.driver=sentinel — with no
// code change and no per-starter key: the center resolves the driver centrally
// and every client picks it up through the same seam. This is the whole point of
// the abstraction/driver split. The bundled zero-dependency "default" driver
// stays available for tests and lightweight use; this module exists so
// production traffic can lean on sentinel-golang's adaptive flow control and
// circuit breaking instead.
package StarterGovernanceSentinel

import (
	"context"

	sentinel "github.com/alibaba/sentinel-golang/api"

	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
)

// SentinelName is the name this backend answers to in the governance document.
const SentinelName = "sentinel"

func init() {
	// Initialise sentinel once at import time. Contribution then always succeeds
	// so a misconfigured environment fails loudly here rather than on first use.
	if err := sentinel.InitDefault(); err != nil {
		panic("starter-governance-sentinel: sentinel init failed: " + err.Error())
	}
	// Contribute the backend as a named bean: the container is the driver
	// directory, and the wiring bean collects every bean
	// exported as resilience.Driver into a name-keyed map. The Export is
	// load-bearing — gs indexes beans by their exact type, so without it the
	// concrete *sentinelDriver would be invisible to that map.
	gs.Provide(func() *sentinelDriver { return &sentinelDriver{} }).
		Name(SentinelName).
		Export(gs.As[resilience.Driver]()).
		Caller(1)
	log.Infof(context.Background(), log.TagAppDef, "registered sentinel resilience driver")
}

type sentinelDriver struct{}

// NewSentinelDriver returns the sentinel-backed [resilience.Driver] as a value,
// for callers that build executors outside the container (examples, tests, a
// program that manages its own wiring). A process using gs does not need it:
// the same backend is contributed as the bean named [SentinelName].
func NewSentinelDriver() resilience.Driver { return sentinelDriver{} }

func (sentinelDriver) NewClientExecutor(service string, p resilience.ClientPolicy) (resilience.ClientExecutor, error) {
	return newSentinelExecutor(service, p)
}

// NewServerExecutor builds the inbound admission executor. Sentinel maps the shared
// knobs (flow rule, circuit breaker, isolation) onto the SAME primitives for both
// directions — its flow rule is keyed by resource, and the resource is the route
// label here — so the driver projects the admission model exactly as the bundled
// one does, and nothing is lost. Sentinel's genuinely inbound-only primitives
// (system rules, hotspot rules) have no [resilience.ServerPolicy] field to arrive
// through yet; when they do, they land in this method.
func (sentinelDriver) NewServerExecutor(service string, p resilience.ServerPolicy) (resilience.ServerExecutor, error) {
	e, err := newSentinelExecutor(service, p.AsPolicy())
	if err != nil {
		return nil, err
	}
	return sentinelServerExecutor{e}, nil
}

// sentinelServerExecutor adapts the sentinel engine — which is defined over
// [resilience.ClientPolicy] — to the inbound seam, whose model is
// [resilience.ServerPolicy]. Only the refresh seam needs the wrapper: the engine is
// the same type, and the admission model is re-projected on each refresh.
type sentinelServerExecutor struct {
	*sentinelExecutor
}

func (e sentinelServerExecutor) Refresh(p resilience.ServerPolicy) error {
	return e.sentinelExecutor.Refresh(p.AsPolicy())
}
