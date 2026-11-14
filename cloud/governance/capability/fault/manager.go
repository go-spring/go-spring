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

// Package fault is the direction-neutral MECHANISM of in-process fault
// injection: the call identity a rule matches ([Target]), the rule core every
// direction embeds ([RuleCore]), the guardrail state model behind a document
// push ([Budget]/[Budgets]), the shared injection sequence ([Kernel]) and the
// runtime record it produces ([Result]).
//
// Configuration is NOT shared. Each direction owns its own rule and config types
// and adds its effect: [go-spring.org/cloud/governance/dataplane/outbound/fault]
// adds an error kind + class, and
// [go-spring.org/cloud/governance/dataplane/inbound/fault] adds the response the
// server returns. A rule therefore cannot express an effect its direction cannot
// produce.
//
// This package depends on nothing of cloud's call vocabulary: traffic
// classification reaches a rule as one more [Target.Metadata] entry, recorded by
// the seam that builds the operation, and is matched by the same subset rule as
// every other targeting dimension.
//
// The files split by role: manager.go holds the call/result vocabulary,
// rule.go the shared rule core and its two computations (selection and dose
// draw), kernel.go the replaceable kernel and guardrail state, and error.go the
// outcome vocabulary.

package fault

import (
	"time"
)

// Target is one call's identity AS A FAULT RULE SEES IT: what was called, which
// interface of it, and the caller's own targeting metadata. A rule is matched
// against these three, so an adapter that can name them can be put on fire —
// which is what lets the same rule vocabulary serve an outbound client call and
// an inbound request (see outbound/fault's Manager.WrapClientExecutor and
// inbound/fault's Server.Fire).
//
// It deliberately carries no instance identity: which replica receives a
// document is decided when the document is DELIVERED, not when a call is matched
// (see [RuleCore]'s doc for why that axis must not return).
//
// It is fault's OWN shape, deliberately not [chain.Operation]: that type is a
// CLIENT-shaped declaration of a call — it carries the trace/metric/log
// declarations and the client-edge kind, none of which a fault rule reads — and
// an inbound request has no meaningful Operation to offer. Passing one was how
// the server seam silently lost its Action and its Metadata (every inbound rule
// degraded to a whole-service match); a Target states only what is matched, so
// a caller cannot omit by accident what a rule needs.
//
// It is NOT configuration: nothing here is written by an operator or bound from
// a {value} tag (that is rule.go's vocabulary). An adapter builds it at call
// time from what its protocol knows, which is the only side that does.
type Target struct {
	// Service is what the call is addressed to — the identity a rule's Service
	// matches by exact compare, and a governance label the operator writes rules
	// against: a client names the downstream ("redis:cache", "http:user-svc"), a
	// server names its own entrance ("gin", "grpc"). Its shape is the starter's to
	// choose, as it is on [chain.Operation.Service]; it must be non-empty, since
	// every rule names a service and none can match an empty one.
	Service string

	// Action is the interface being invoked, in the protocol's own format — the
	// second filter after Service, and the one that makes INTERFACE-level fault
	// injection possible: fire one endpoint, one method, instead of a whole
	// service. Empty means "no interface named", which only a rule that names no
	// Action matches.
	//
	// The format is deliberately not fixed, because "the action" differs by
	// protocol: an RPC names its method ("/pkg.Svc/Method", trpc's callee method),
	// an HTTP server names METHOD + path ("GET /users/{id}"), exactly as
	// [chain.Request.Action] does — the string an interface-precise rule needs.
	Action string

	// Metadata is the call's key-value targeting set — the third filter, and the
	// one that separates two calls to the SAME service: a rule's Match must be a
	// subset of this map (see [RuleCore.MetadataMatch]). It is filled by the seam
	// that builds the operation — the base constructors record the call's traffic
	// class under the ordinary key "traffic" — and is matched, never emitted, so
	// its keys are whatever dimensions the deployment cares about ("tenant",
	// "region", "canary").
	//
	// Nil (the common case) means the call declares nothing, and only rules keyed
	// on service and action alone can match it.
	Metadata map[string]string
}

// Result is what one attempt drew from the rule that governed it: which drill
// ([RunID]), which rule ([Name]), and the dose itself — an injected error
// ([Inject]/[Error]/[Class]) and/or a latency ([Delay]). There is deliberately
// no nested "effect" type: the dose never travels apart from its attribution,
// so one flat value says the whole sentence.
//
// It is the SHARED RUNTIME RECORD, not configuration: both directions report it
// to their observers, and the outbound chain keeps it on the round. Each
// direction's rule decides what [Error]/[Class] mean; an inbound response does
// not use them the way an outbound error does.
//
// Rules are an ORDERED list: the first one whose conditions match governs the
// attempt and evaluation stops there. Later rules never run, so a result
// carries AT MOST one rule's dose and one call is never affected by more than
// one experiment. Order IS precedence among rules on the same service — put
// the specific ones first.
//
// A governing rule that does not fire (Rate draw missed, guardrail tripped,
// blast radius full) still terminates selection but leaves the attempt untouched:
// the result is then the zero value. Matching decides governance; the draw
// decides whether the fault is applied, and there is no fall-through to the
// next match.
//
// It is delivered PER ATTEMPT, not per call: the kernel runs once inside the
// resilience executor's retry loop, so a call that retried produces one Result
// per attempt and nothing in this package aggregates them. A caller that needs
// "what happened to this call" must read the rounds (which is what the client
// seam writes each of these into, and what the observe kit reads back); a caller
// that captures only the LAST Result will miss a fire that happened on an
// earlier attempt and lost nothing on the final one.
//
// It carries no error: whether the call may proceed is the ERROR the kernel
// returns, so there is exactly one place to look and no second copy that could
// disagree.
type Result struct {
	// RunID is the direction document's RunID — the drill execution this result
	// belongs to, empty when the document carries none. It is what lets a span, a
	// call record or a counter snapshot be grouped into "this drill" across
	// replicas, since the rule's Name is the rule's stable identity, not a run.
	RunID string

	// Name is the governing rule's configured Name — the rule's identity
	// and the key a platform joins telemetry on. Empty on the zero value.
	Name string

	// Inject reports whether the governing rule drew the effect for this attempt.
	Inject bool

	// Error is the injected error's kind ([ErrorGeneric], [ErrorTimeout],
	// [ErrorReset], [ErrorRefused]) — the word the call record and the observe
	// tags carry. Empty unless Inject. It is an OUTBOUND shape; an inbound
	// response rule fills its own effect instead.
	Error string

	// Class is the failure class an outbound rule declared ([ClassTransient],
	// [ClassPermanent]); empty means [ClassTransient]. An inbound injection has
	// no class — it declares [Status] instead.
	Class string

	// Status is the inbound response status a server rule declared (400–599):
	// the code the server answers with instead of running the handler. Zero
	// when no inbound response was injected (an outbound rule never sets it).
	Status int

	// Body is the response message a server rule declared, used with [Status].
	// HTTP adapters write it as the response body, RPC adapters use it as the
	// status message. Empty means "no declared body".
	Body string

	// ContentType is the HTTP content type for [Body]. HTTP-only; an RPC
	// adapter ignores it.
	ContentType string

	// Delay is the latency the rule applied, already including the jitter draw.
	// It is the sleep that was ASKED for: when the attempt's context is cancelled
	// mid-sleep, the result still reports the requested delay and the kernel
	// returns the context error.
	Delay time.Duration
}

// Fired reports whether this attempt was actually affected — an effect injected
// or a latency applied. A cancelled latency still counts as fired: the fire was
// set, the call was affected, and the call record carries it.
func (r Result) Fired() bool { return r.Inject || r.Delay > 0 }
