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
)

// rule.go is the OUTBOUND configuration vocabulary: targeting and dose come from
// the shared [kfault.RuleCore]; the effect is the outbound error kind and the
// class the resilience stack reads. An inbound rule cannot express these, and
// this rule cannot express an inbound response — the direction split is in the
// types.

// ClientRule is one OUTBOUND fault rule: it names one service (optionally one
// interface and one traffic slice of it) and the failure to inject. See
// [ClientConfig.Rules].
type ClientRule struct {
	kfault.RuleCore

	// Error selects the injected failure kind, applied at Rate:
	//   [kfault.ErrorGeneric] — a retryable injected error ([kfault.ErrInjected]), the default
	//   [kfault.ErrorTimeout] — wraps os.ErrDeadlineExceeded (an I/O read/write timeout)
	//   [kfault.ErrorReset]   — wraps syscall.ECONNRESET
	//   [kfault.ErrorRefused] — wraps syscall.ECONNREFUSED
	// Rate 0 means no errors are injected (a latency-only rule).
	Error string `value:"${error:=generic}" expr:"$ in ['generic', 'timeout', 'reset', 'refused']"`

	// Class is how the caller's resilience stack classifies the injected failure:
	//   [kfault.ClassTransient] (default) — retried, counted by the circuit breaker
	//   [kfault.ClassPermanent]           — NOT retried, still counted
	// There is no "business" value: the outbound chain never sees a business
	// answer (a client adapter folds one to nil with resilience.Tolerate before
	// the executor runs), so the expr tag rejects it.
	//
	// It is the process-internal counterpart of the status code a wire-level chaos
	// tool can inject: what the stack actually consumes is the classification, not
	// the number. Use "permanent" to exercise "this failure must not be retried".
	//
	// WHY THE CLASS RIDES ON THE RULE (not on some caller-side list): this
	// framework's classification contract is IDENTITY-DRIVEN — [resilience.Retryable]
	// is an interface an error implements to answer for itself (a non-idempotent
	// write opts out of retry the same way), in the Hystrix typed-exception
	// tradition. There is no resilience4j-style ignore/record exception list for
	// a synthetic error to fall through to, so a class an injection cannot carry
	// is a class nothing else could express: dropping this field would make the
	// "permanent, not retried" path unreachable, with no alternative entrance.
	// [kfault.InjectedError] carrying that class is therefore not the injection
	// overriding the framework — it is the injected error obeying the same
	// contract every real error obeys.
	Class string `value:"${class:=transient}" expr:"$ in ['transient', 'permanent']"`

	// Status, when set, gives the injected failure a protocol status (400–599):
	// the code the caller would have received from a real answer. It is
	// exported as [kfault.InjectedError.Status] (and [kfault.StatusOf]) for the
	// adapter to interpret — a client library whose failure IS an error
	// (gRPC-like protocols) can translate it into its native error. `net/http`
	// is deliberately different: a 5xx there is a RESPONSE, not an error, so an
	// HTTP caller sees a status only once response replacement lands.
	// 0 (the default) leaves the error protocol-neutral.
	Status int `value:"${status:=0}" expr:"$ == 0 || ($ >= 400 && $ <= 599)"`
}

// Core implements [kfault.Rule].
func (r ClientRule) Core() *kfault.RuleCore { return &r.RuleCore }

// Roll implements [kfault.Rule]: it attaches the outbound error kind and class
// to the dose the kernel decided.
func (r ClientRule) Roll(d kfault.Dose) (kfault.Result, error) {
	o := kfault.Result{Name: r.Name, Delay: d.Delay}
	if !d.Inject {
		return o, nil
	}
	o.Inject = true
	o.Error = r.Error
	o.Class = r.Class
	o.Status = r.Status
	return o, kfault.NewInjectedError(r.Error, r.Class, r.Status)
}

// ClientConfig is the set of rules ARMED on the outbound side right now — an
// experiment's projection onto this process, not the experiment (the plan, its
// scheduling and its stop conditions live in the platform). One push may carry
// many rules; they all belong to the same RunID.
//
// There is deliberately NO global default and NO catch-all: a call is set on
// fire only when a rule that NAMES its service matches it. An empty config
// injects nothing, so a client with no rule armed is a transparent no-op.
//
// Bind via indexed properties, e.g.:
//
//	spring.fault.client.rules[0].name=redis-errors
//	spring.fault.client.rules[0].service=redis:cache
//	spring.fault.client.rules[0].rate=0.5
type ClientConfig struct {
	// RunID labels ONE DRILL EXECUTION of this document — the id a control plane
	// mints when an operator starts a drill and every replica shares. It is
	// document metadata, not a rule and not an effect. Empty is legal on the
	// direct path and means "no drill id".
	//
	// It is NOT a configuration field: there is no binding tag, so an operator
	// cannot set it per direction. The fault root assigns it from the document's
	// RunID before it commits; a caller that drives Refresh directly sets it in
	// code.
	RunID string

	// Rules are an ORDERED list. The FIRST rule whose conditions match a call
	// governs it — evaluation stops there, so a call is never affected by more
	// than one rule. Order is precedence among rules on the SAME service.
	Rules []ClientRule `value:"${rules:=}"`
}

// toRules boxes the typed rules for the shared kernel.
func toRules(rs []ClientRule) []kfault.Rule {
	out := make([]kfault.Rule, len(rs))
	for i := range rs {
		out[i] = rs[i]
	}
	return out
}
