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

// operation.go is the DECLARATION half of a call: everything the caller states
// before the call runs, and nothing the chain learns while it does. It travels
// by value and is immutable by construction; what the call comes back with is
// written into [Recorder], in recorder.go.

package chain

import (
	"strings"

	"go-spring.org/log"
	"go-spring.org/stdlib/strutil"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Kind is the SHAPE of interaction a governed call has, independent of the wire
// protocol that carries it. There are exactly three, and they are exhaustive:
// waiting for an answer ([KindCall]), handing something over without waiting
// ([KindPublish]), and taking something in to handle it ([KindConsume]). It
// exists so a starter declares the shape once, instead of each one re-deriving
// the trace edge from its own strings.
//
// It is one of three orthogonal coordinates, and the least specific of them:
// [Kind] is the SHAPE of the interaction, [Request.System] is the WIRE it rides
// ("redis", "kafka", "gin"), and [Request.Action] is the protocol's own name for
// what was done ("GET /users/{id}", "insert", "send"). Because shape and wire are
// independent, their cross product is expressible: a request/response exchange
// over a broker is [KindCall] with a messaging system, a fire-and-forget POST is
// [KindPublish] with an HTTP system.
//
// For MESSAGING the two vocabularies happen to live in the same domain, which is
// the one place they look redundant. OTel names a messaging operation with a
// finer word than a shape — send / receive / process / settle — and that word
// belongs in [Request.Action]; this type is its COARSE PROJECTION, the part the
// trace edge can be picked from without knowing the protocol: send is the
// producer edge and process the consumer edge, while receive (a pull from the
// broker) and settle (its acknowledgement) are client-side calls, which
// [KindCall] already describes. Elsewhere the two have nothing to do with each
// other — "INSERT" says nothing about which edge it adds.
//
// It is CLIENT-SIDE. An inbound request is handled by the server stack, which
// derives [trace.SpanKindServer] itself and reads nothing here; the zero value
// is [KindCall], so the field is simply meaningless on an inbound Operation
// rather than wrong.
type Kind uint8

const (
	// KindCall is a synchronous request/response call: the caller waits for an
	// answer. It is the zero value — the shape an ordinary client call has always
	// had — so a client that declares nothing keeps it.
	KindCall Kind = iota

	// KindPublish is a one-way send into a broker: the call hands a message over
	// and returns when the broker accepts it. It is the producer edge of a trace.
	KindPublish

	// KindConsume is a delivery taken out of a broker: the call is the handling of
	// a message the broker produced. It is the consumer edge of a trace.
	KindConsume
)

// String names the direction for logs and labels.
func (k Kind) String() string {
	switch k {
	case KindPublish:
		return "publish"
	case KindConsume:
		return "consume"
	default:
		return "call"
	}
}

// SpanKind maps the base action onto the trace edge it adds. A call is the
// client edge of a request/response pair (this side waits for the other side's
// server edge), a consume is the consumer edge of a delivery, and a publish is
// the producer edge itself.
//
// KindCall is [trace.SpanKindClient] — NOT internal. The governed calls this
// package describes reach a database, a broker or another service, and a backend
// that pairs client and server edges (the service graph of any trace UI) can only
// do so if the outbound edge says it is one. A genuinely in-process call states
// [Observe.Span.Kind] = [trace.SpanKindInternal] itself, which is the one case the
// derived default cannot know about.
//
// Whichever kind results, exactly ONE layer opens the span for a given protocol:
// this one when the client declares an operation, the protocol's own
// instrumentation (an otelhttp transport, a tracing driver) otherwise. Two
// instrumentations over one call produce two spans — one nested in the other —
// and double-counted metrics.
func (k Kind) SpanKind() trace.SpanKind {
	switch k {
	case KindPublish:
		return trace.SpanKindProducer
	case KindConsume:
		return trace.SpanKindConsumer
	default:
		return trace.SpanKindClient
	}
}

// Request is WHAT a call is: the base action it performs (call/publish/consume),
// the wire system it rides, the interface it invokes, and the caller's
// targeting metadata. Together these are the coordinates a rule matches on —
// governance and fault match on [Request.Action] and [Request.Metadata] — and the
// protocol's own vocabulary for naming what happened.
//
// It is the call's IDENTITY, and it stops there: how the call's signals are
// named, and what they carry, is [Observe]'s business, declared beside it. The
// line between the two is the READER — [Request.Action] is matched by a rule as
// well as emitted, and [Request.Metadata] is matched and never emitted, while a
// span attribute is read by the emitter alone and lives in [Observe].
//
// Two sides fill it, which is why its fields are not all of one kind: the
// protocol package fills [Request.Kind], [Request.System] and [Request.Action] — it is
// the only side that knows whether these calls reach a broker, a database or an
// HTTP service — while the caller, or a middleware reading request headers,
// fills [Request.Metadata].
type Request struct {
	// Kind is the base action. Its zero value is [KindCall], so an ordinary client
	// call declares nothing here.
	Kind Kind

	// Action is the finest addressable name of this operation — the interface
	// being invoked — in the PROTOCOL'S OWN format. The format is deliberately not
	// fixed here, because "the action" differs by protocol:
	//
	//   - a request/response RPC or a database call names the operation word:
	//     "get", "insert", "publish", "Search" — a bounded set.
	//   - an HTTP/REST call names METHOD + path ("GET /users/{id}"), because the
	//     method alone cannot tell two RESTful endpoints apart — and this is
	//     exactly the string an interface-precise governance rule needs.
	//
	// It reaches the span name and the access log, and it is the string a rule
	// matches on to target one interface — the one field here that the emitter
	// does not have to itself. It is not, by itself, a metric label: the bounded
	// part of the same idea (the method, the operation word) is what labels a
	// metric, and it belongs in [Observe.Metric.Attrs].
	Action string

	// System is the wire protocol family's value — "redis", "kafka", "gorm",
	// "gin" — the shared vocabulary of the family, not a per-starter choice. It
	// is the family's identity, which is why it sits here and not in [Observe]:
	// what labels a signal's `system` is the family's own attribute, written into
	// [Observe.Metric.Attrs] / [Observe.Span.Attrs] / [Observe.Log.Fields].
	System string

	// Metadata is the caller's own key-value targeting set, used to pick WHICH
	// governance or fault rule governs this call — the third filter after
	// [Operation.Service] and [Request.Action]. It is filled by the CALLER (or a
	// middleware, often from request headers) and is matched, never emitted: a
	// rule carries a metadata selector over it
	// ([go-spring.org/cloud/runtime/dataplane/kernel/matcher]) and the first
	// rule whose selector is a subset of this map wins.
	//
	// It is deliberately opaque to this package: it holds whatever dimensions the
	// deployment cares about ("tenant", "region", "canary", …), and a small
	// vocabulary of reserved keys is a documentation convention, not a type. It
	// does NOT partition protection state — state is keyed by the matched rule,
	// bounded — and it does not become a metric label, because its values are
	// drawn from an open set.
	//
	// Nil (the common case) means the call declares no targeting metadata and only
	// rules keyed on the service (and action) alone can match it.
	Metadata map[string]string
}

// Observe is the OBSERVABILITY block of an Operation: everything about a call's
// signals, grouped BY SIGNAL TYPE — [Observe.Log], [Observe.Span],
// [Observe.Metric] — with nothing else.
//
// It is DECLARATION ONLY. Everything here is stated before the call runs, which
// is what lets it travel by value inside [Operation]: nothing the call learns
// lands here. The facts the chain measures as the call runs — the rounds, their
// durations, the injected faults — are written into the separate [Recorder];
// what the call has to ADD to these three groups is written into its own block
// ([Recorder.Observed]), which is the same three groups filled the other way.
// Keeping the static half static is what makes an Operation safe to share, cache
// and reuse.
//
// The declared attributes are PER DESTINATION, one list per group: a value a
// call wants on a metric goes in [Metric.Attrs], one it wants on the span in
// [Span.Attrs], one it wants in the log in [Log.Fields]. This is deliberately
// redundant — a value useful in more than one place is written into each list —
// and it buys the caller full freedom: it can put a value on the span without
// also labelling a metric, or label a metric without also logging it. The price
// is that the SAME fact is spelled more than once, so the lists must be kept in
// step by hand (nothing here derives one from another).
//
// The one hard rule the split enforces: [Metric.Attrs] values MUST be bounded.
// A metric label whose values are drawn from an open set (a key, a statement, a
// topic, a path) multiplies the series until the metric is unusable, so an
// open-set value belongs in [Span.Attrs] and/or [Log.Fields] instead.
type Observe struct {

	// Log is the access-log group: the tag, the level policy and the fields.
	Log Log

	// Span is the trace group: the kind override, the attributes and the links.
	Span Span

	// Metric is the metric group: the family prefix and the labels.
	Metric Metric
}

// Log is the ACCESS-LOG group of [Observe]: how the call's access line is
// tagged, levelled and filled.
type Log struct {
	// Tag is the tag the access log for this operation is written under. It is
	// the client's, not the emitter's, so a service's existing access log keeps
	// its identity across the move to a single emitter.
	Tag *log.Tag

	// QuietSuccess levels the call's SUCCESS access line: true logs it at Debug,
	// so a high-frequency operation (a cache GET, a health PING) does not flood
	// the log with uninteresting lines until it fails; false logs it at Info. A
	// failure is always logged at Warn regardless.
	QuietSuccess bool

	// Fields are the fields that become ACCESS-LOG fields, verbatim. They are
	// [log.Field] rather than an attribute, so this list is also the escape
	// hatch for a field with no metric or span meaning at all.
	//
	// The list carries the REQUEST half of the access line only: each call's own
	// argument — "key", "sql", "topic", or a pipeline's command list — filled
	// when the Operation is built. The log has no cardinality limit, so an
	// open-set value is fine here, the very reason it must NOT go into
	// [Metric.Attrs]. The RESPONSE half does NOT belong here: only the fn knows
	// what the response carried, and it appends those fields to
	// [Recorder.Observed.Log] — one line per call, both halves — rather than to a
	// list here, which a shared Operation would accumulate one copy of per call.
	//
	// A value that differs PER ROUND — the item that failed on this attempt — is
	// neither: the access line is one line for the whole call, so such a value has
	// to be summarised there rather than appended once per attempt.
	Fields []log.Field
}

// Span is the TRACE group of [Observe]: the call's edge in a trace and what it
// carries.
type Span struct {
	// Kind overrides the kind the emitter derives from [Request.Kind]. It is
	// normally zero (unspecified), meaning "use the direction's kind"; it exists
	// for the rare call whose trace edge is not implied by its action.
	Kind trace.SpanKind

	// Attrs are the attributes that become SPAN attributes. They may be bounded
	// or open-set (a key, a statement, a topic); the span has no cardinality
	// limit, so this is where unbounded per-call detail lives.
	Attrs []attribute.KeyValue

	// Links are the trace links this call adds to its span — the association a
	// consumer span keeps to the producer span that created the message it is
	// handling, which a parent/child edge cannot express because the producer
	// may live in another trace. Empty for a plain call.
	Links []trace.Link
}

// Metric is the METRIC group of [Observe]: the family a call reports under and
// the labels it reports with.
type Metric struct {
	// Name is the metric family name a client-side operation of this kind
	// reports under, without the trailing dot: "db.client" for a database
	// client, "messaging.client" for a message broker client. It is both the
	// emitter's instrument prefix and the group's marker that the call is
	// DESCRIBED at all: an empty Name means the call declared no telemetry and
	// takes the emitter's generic fallback.
	//
	// It must not be empty for a described call, and it must be a legal OTel
	// instrument name: the emitter builds its instrument names from it, so a
	// family that is neither leaves every metric for that client unregistered.
	// That failure is reported (at Warn, once per family) rather than swallowed,
	// because a typo here deletes a whole metric surface without any other
	// symptom.
	Name string

	// Attrs are the attributes that become METRIC LABELS. They MUST be bounded
	// — see [Observe]'s note. A value wanted on the span or in the log as well
	// is added to [Span.Attrs] / [Log.Fields] too; nothing does that
	// automatically.
	Attrs []attribute.KeyValue
}

// Described reports whether this group declares a description at all: a non-empty
// family is the marker, since a client that declares nothing takes the emitter's
// fallback signals. It is stated here, on the block that owns the marker, rather
// than re-tested wherever a reader needs to know.
func (m Metric) Described() bool { return m.Name != "" }

// Operation is the description of one governed call: what it is ADDRESSED TO
// ([Operation.Service]), what it DOES and what it is TARGETED BY ([Request]),
// how to name the signals it produces ([Observe]), and its retry declaration.
//
// It is the single descriptor passed to [Executor.Execute], unifying what a
// call IS with how the process should DESCRIBE it — both declared by the client
// that owns the call, the only side that knows whether it is talking to a cache,
// a broker or a database — and both travelling to every link of the chain
// through the Execute signature.
//
// It is DECLARATION ONLY, and it travels BY VALUE: the caller builds it, hands
// it over, and the call's outcome comes back in a separate [Recorder]. That
// split is the package's POINTER RULE — a value is immutable by construction (a
// layer that mutates its own copy changes nothing anyone else sees), while the
// one thing the chain must write into, the record, is passed explicitly as a
// pointer whose nil means "record nothing". Nothing the call learns is stored
// here, so one Operation can be shared, cached and reused across calls without
// any of them seeing another's facts.
type Operation struct {
	// Service is the identity of what the call is ADDRESSED TO — the one
	// coordinate protection, policy resolution and rule matching all key on. Its
	// value's SHAPE is the protocol's to choose, because "the target" differs by
	// protocol:
	//
	//   - a call to a NAMED downstream service (http, grpc, dubbo) names that
	//     service — "user-svc" — the logical target, which doubles as the
	//     governance label a rule is written against.
	//   - a call to an infrastructure dependency (redis, mysql, kafka) has no
	//     logical layer above the concrete instance, so it names the dependency
	//     itself — a configured name when it has one ("redis:cache"), else the
	//     address it was configured with ("redis:10.0.0.1:6379"). Two instances
	//     (two redis pools) must not collapse into one series, and naming them
	//     apart is what keeps them apart.
	//
	// It is ONE coordinate, not two: for both shapes it is simply "the thing
	// being called". There is deliberately no second "logical service vs physical
	// instance" axis — service discovery and load balancing are loadbalance's and
	// discovery's to resolve, below this descriptor, so what reaches a rule, a
	// breaker and a metric is the one name the deployment configured. What
	// distinguishes two calls to the SAME target is not a second service axis but
	// [Request.Metadata], and only where a rule chooses to look at it.
	//
	// The CONCRETE instance a round reached is not declared here: it is discovered
	// per round (a retry may be balanced onto another node), so it cannot be stated
	// before the call runs.
	//
	// It is the CALL's identity and is expected to be set: a client names the
	// target it is built for, and a layer that reads this Operation (the fault
	// layer, a governance rule) keys on it. An executor that was built for one
	// service may fall back to its own label for a call that leaves it empty, but
	// a caller that can state it should.
	Service string

	// Request is what the call asks for, in the protocol's vocabulary. See
	// [Request].
	Request Request

	// Observe is the observability block: the call's signals, DECLARED. The
	// facts the chain measures as the call runs are written into the [Recorder]
	// the caller passes beside this Operation, not here. See [Observe].
	Observe Observe

	// Retry is the call's retry declaration — the explicit channel through which
	// the caller (the only side that knows whether repeating the call repeats a
	// side effect) opts into retry. The zero value means "no retry": a call runs
	// exactly once unless someone declares otherwise, because retry is the one
	// protection stage that amplifies a downstream's load rather than gating it.
	//
	// It travels per call, unlike the rest of a service's protection, which is
	// construction-bound on the executor. A governance center MAY still push a
	// retry node on the policy for the whole service — that node overrides this
	// one whole when it declares a schedule (a live hot-fix must win), while the
	// NonIdempotent flag is preserved from both (see the resilience package's
	// EffectiveRetry, which is the single place the two are merged).
	Retry RetryPolicy
}

// ActionMatch is a rule's match condition over a call's [Request.Action] — the
// interface being invoked ("get", "insert", "publish", or "GET /users/{id}" for
// a REST endpoint). Its zero value matches every action, so a rule that declares
// neither field governs the whole service.
//
// Exact is equality; Prefix matches a leading string, which is how one rule
// covers a REST subtree whose concrete paths vary ("GET /users" covers
// "GET /users/{id}"). They are mutually exclusive in practice — Exact wins when
// both are set — so a rule states one intent.
type ActionMatch struct {
	Exact  string `value:"${exact:=}"`
	Prefix string `value:"${prefix:=}"`
}

// Matches reports whether action satisfies m.
func (m ActionMatch) Matches(action string) bool {
	switch {
	case m.Exact != "":
		return action == m.Exact
	case m.Prefix != "":
		return strings.HasPrefix(action, m.Prefix)
	default:
		return true
	}
}

// maxDetail bounds a value captured as an open-set detail attribute. A topic, a
// statement or a path can be long and neither a span nor a log line has any use
// for all of it.
const maxDetail = 512

// Metric prefixes the two client families report under. The family — not the
// starter — owns its prefix: only the family knows whether these calls are a
// database or a broker.
const (
	metricDB        = "db.client"
	metricMessaging = "messaging.client"
	metricHTTP      = "http.client"
)

// attr couples one attribute with its log field, for the ubiquitous case where a
// value belongs on the metric, the span and the log alike. It exists so the key
// is spelled once in a constructor — a value the three destinations share cannot
// drift apart. A protocol that wants the destinations to differ writes the lists
// directly.
func attr(key, val string) (attribute.KeyValue, log.Field) {
	return attribute.String(key, val), log.String(key, val)
}

// MessagingOp is the operation a publish or a consume of system constructs,
// addressed by destination. kind is the interaction shape and its trace edge;
// system is the backend's family value ("kafka", "pulsar", "asynq"); operation
// is the messaging operation word the call reports as its messaging.operation
// ("send", "receive", "process", "settle" — the OTel vocabulary, or the word the
// client's own API uses when it has a better one, like asynq's "enqueue").
//
// Pass the word the CLIENT's API actually names the operation with: an empty one
// falls back to the shape's own word ([Kind.String], i.e. "publish"/"consume"),
// which is the coarsest true statement about the call — a valid default, but a
// lossy one. The finer word is what separates the poll that fetched a batch from
// the handler that processed one message from the commit that settled it, and it
// is also what an interface-precise governance rule matches on.
//
// destination is the topic, queue or task type the call addresses. Pass it ONLY
// when the call targets exactly ONE bounded destination: a batch that spans
// several topics, or a call with no destination at all, passes "" — and the
// span then names the operation alone, which is what OTel prescribes (omit the
// destination when no low-cardinality one is available). Naming a mixed batch
// after its first element would be a lie told in the span NAME and in the
// governance key, not merely in a detail field, so this is a contract and not a
// nicety.
//
// The span is named `{operation} {destination}`, the order the messaging
// conventions specify (the older `{destination} {operation}` was reordered);
// [HTTPOp] names its span the same way round, `METHOD path`.
//
// system, the operation word and the shape are bounded, so they label the
// metric, the span and the log. The destination is drawn from an open set: it
// reaches the span name, the span attributes and the log, but never a metric
// label — a topic per series would multiply the metric until it is unusable.
func MessagingOp(kind Kind, system, operation, destination string) Operation {
	if operation == "" {
		operation = kind.String()
	}
	sys, sysF := attr("messaging.system", system)
	opv, opF := attr("messaging.operation", operation)
	tel := Observe{
		Metric: Metric{Name: metricMessaging, Attrs: []attribute.KeyValue{sys, opv}},
		Span:   Span{Attrs: []attribute.KeyValue{sys, opv}},
		Log:    Log{Fields: []log.Field{sysF, opF}},
	}
	// The span's name, and the string a governance rule matches on. It is NOT
	// truncated: a rule written against the whole name must be able to match it,
	// so length is controlled at the CALL — by passing no destination when it is
	// not unique — rather than by cutting the match key.
	action := operation
	if destination != "" {
		dest := strutil.Truncate(destination, maxDetail)
		tel.Span.Attrs = append(tel.Span.Attrs, attribute.String("messaging.destination.name", dest))
		tel.Log.Fields = append(tel.Log.Fields, log.String("messaging.destination.name", dest))
		tel.Log.QuietSuccess = true
		action = operation + " " + destination
	}
	return Operation{
		Request: Request{Kind: kind, System: system, Action: action},
		Observe: tel,
	}
}

// DBOp is the operation a database call of system constructs: operation is the
// bounded operation word ("get", "exec", "query", "GET" for an HTTP-fronted
// store), statement is the open-set statement, key or path the call ran against,
// empty when the client has none.
//
// A system whose wire is HTTP (an object store, a metrics API) still describes
// itself as a database here: the family is defined by what the call MEANS — a
// request against a store — not by the transport that carries it.
func DBOp(system, operation, statement string) Operation {
	sys, sysF := attr("db.system", system)
	opv, opF := attr("db.operation", operation)
	tel := Observe{
		Metric: Metric{Name: metricDB, Attrs: []attribute.KeyValue{sys, opv}},
		Span:   Span{Attrs: []attribute.KeyValue{sys, opv}},
		Log:    Log{Fields: []log.Field{sysF, opF}},
	}
	if statement != "" {
		stmt := strutil.Truncate(statement, maxDetail)
		tel.Span.Attrs = append(tel.Span.Attrs, attribute.String("db.statement", stmt))
		tel.Log.Fields = append(tel.Log.Fields, log.String("db.statement", stmt))
		tel.Log.QuietSuccess = true
	}
	return Operation{
		Request: Request{System: system, Action: operation},
		Observe: tel,
	}
}

// HTTPOp is the operation an outbound HTTP call constructs: method is the HTTP
// verb, route is the request target (a path, a URL). The method is bounded, so it
// labels the metric, the span and the log; the route is drawn from an open set,
// so it reaches only the span and the log.
func HTTPOp(method, route string) Operation {
	m, mF := attr("http.request.method", method)
	tel := Observe{
		Metric: Metric{Name: metricHTTP, Attrs: []attribute.KeyValue{m}},
		Span:   Span{Attrs: []attribute.KeyValue{m}},
		Log:    Log{Fields: []log.Field{mF}},
	}
	if route != "" {
		full := strutil.Truncate(route, maxDetail)
		tel.Span.Attrs = append(tel.Span.Attrs, attribute.String("url.full", full))
		tel.Log.Fields = append(tel.Log.Fields, log.String("url.full", full))
		tel.Log.QuietSuccess = true
	}
	return Operation{
		Request: Request{System: "http", Action: method + " " + route},
		Observe: tel,
	}
}

// Described reports whether the operation declares a description at all — a
// non-empty metric family is the marker, because a client that declares nothing
// (a skipped PING, a driver's own silent call) takes the emitter's fallback
// signals, whose names this layer has always used in the undeclared case.
//
// The marker lives on the group ([Metric.Described]) rather than here, so the
// convention is stated once and a reader of the Metric block finds it there.
func (op Operation) Described() bool { return op.Observe.Metric.Described() }

// SpanKind is the span kind for the call: the one [Observe] overrides, else
// the one derived from the base action [Request].Kind. It is
// [trace.SpanKindInternal] for an ordinary call, so a client that declares no
// direction keeps the kind an in-process call has always had.
func (op Operation) SpanKind() trace.SpanKind {
	if op.Observe.Span.Kind != trace.SpanKindUnspecified {
		return op.Observe.Span.Kind
	}
	return op.Request.Kind.SpanKind()
}
