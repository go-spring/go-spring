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

import (
	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/stdlib/errutil"
)

// rule.go is the INBOUND configuration vocabulary: targeting and dose come from
// the shared [kfault.RuleCore]; the effect is the response status this server
// answers with instead of running the handler. An outbound rule cannot express
// an inbound response, and this rule cannot express an outbound error kind — the
// direction split is in the types.

// ServerRule is one INBOUND fault rule: it names one entrance (optionally one
// interface and one traffic slice of it) and the response status this server
// returns instead of running the handler.
type ServerRule struct {
	kfault.RuleCore

	// Status is the HTTP status this server returns, applied at Rate — an
	// ERROR answer, not a successful one: 400–599 only (2xx/3xx replacement is
	// deliberately out of scope for now). The handler does not run, so this is
	// "the server answered this" rather than "a real response was corrupted".
	//
	// It replaces the whole handler response, so an error status is
	// 503-equivalent, etc.; the adapters that are not HTTP map the FAMILY of the
	// status (4xx → caller-error code, 5xx → unavailable code) onto their own
	// vocabulary.
	//
	// Required when Rate > 0: a rule that would fire without saying what to
	// answer is a configuration error, rejected by [ServerConfig.Validate].
	// 0 means "unset" (a latency-only rule).
	Status int `value:"${status:=0}" expr:"$ == 0 || ($ >= 400 && $ <= 599)"`

	// Body is the message accompanying Status. HTTP adapters write it as the
	// response body; RPC adapters (gRPC/trpc/dubbo) use it as the status message.
	// Empty means an empty HTTP body / the injected error's own text.
	Body string `value:"${body:=}"`

	// ContentType is the HTTP content type for Body (e.g. application/json).
	// HTTP-only; an RPC adapter ignores it. It requires Body
	// ([ServerConfig.Validate]).
	ContentType string `value:"${content-type:=}"`
}

// Core implements [kfault.Rule].
func (r ServerRule) Core() *kfault.RuleCore { return &r.RuleCore }

// Roll implements [kfault.Rule]: it records the response status on the dose the
// kernel decided. It returns an error only to short-circuit the handler — the
// inbound adapter renders the response from [kfault.Result.Status].
func (r ServerRule) Roll(d kfault.Dose) (kfault.Result, error) {
	o := kfault.Result{Name: r.Name, Delay: d.Delay}
	if !d.Inject {
		return o, nil
	}
	o.Inject = true
	o.Status = r.Status
	o.Body = r.Body
	o.ContentType = r.ContentType
	return o, kfault.NewInjectedError(kfault.ErrorGeneric, "", r.Status)
}

// ServerConfig is the set of rules ARMED on the inbound side right now — an
// experiment's projection onto this process, not the experiment (the plan, its
// scheduling and its stop conditions live in the platform). One push may carry
// many rules; they all belong to the same RunID.
//
// There is deliberately NO global default and NO catch-all: a request is set on
// fire only when a rule that NAMES its entrance matches it. An empty config
// injects nothing, so a server with no rule armed is a transparent no-op.
//
// Bind via indexed properties, e.g.:
//
//	spring.fault.server.rules[0].name=gin-errors
//	spring.fault.server.rules[0].service=gin
//	spring.fault.server.rules[0].rate=0.5
type ServerConfig struct {
	// RunID labels ONE DRILL EXECUTION of this document — the id a control plane
	// mints when an operator starts a drill and every replica shares. It is
	// document metadata, not a rule and not an effect. Empty is legal on the
	// direct path and means "no drill id".
	RunID string

	// Rules are an ORDERED list. The FIRST rule whose conditions match a request
	// governs it — evaluation stops there, so a request is never affected by
	// more than one rule. Order is precedence among rules on the SAME entrance.
	Rules []ServerRule `value:"${rules:=}"`
}

// Validate rejects a rule that could fire without saying what to answer: every
// rule with Rate > 0 must declare a status. Field-level shape (400–599) is the
// field's own expr tag; this is the one cross-field constraint.
func (c *ServerConfig) Validate() error {
	for _, r := range c.Rules {
		if r.Rate > 0 && r.Status == 0 {
			return errutil.Explain(nil,
				"fault: server rule %q declares rate > 0 without a status (400-599)", r.Name)
		}
		if r.ContentType != "" && r.Body == "" {
			return errutil.Explain(nil,
				"fault: server rule %q declares content-type without a body", r.Name)
		}
	}
	return nil
}

// toRules boxes the typed rules for the shared kernel.
func toRules(rs []ServerRule) []kfault.Rule {
	out := make([]kfault.Rule, len(rs))
	for i := range rs {
		out[i] = rs[i]
	}
	return out
}
