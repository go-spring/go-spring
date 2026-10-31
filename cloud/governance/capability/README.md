# capability

[English](README.md) | [中文](README_CN.md)

`capability` is the **plane-neutral tier** of the governance runtime: mechanisms
and concepts that belong to neither the control plane nor the data plane, but
that both consume. The control plane's documents embed their types; the data
plane's directions execute with them.

## The rule

> A package belongs in `capability` when it answers a state, vocabulary or
> mechanism question — addresses, endpoints, failure state, budget, traffic
> class, service naming, matching — and knows neither a call direction nor a
> document source.

Once code starts speaking `Executor`, `Operation`, `Recorder`, `Request`,
`Admit` or `Done`, it knows its direction and lives under
[../dataplane/outbound](../dataplane/outbound/) or
[../dataplane/inbound](../dataplane/inbound/) instead. Once it fetches,
validates or distributes a document, it belongs to [../controlplane/](../controlplane/).

This is also a dependency invariant:

> **`capability` imports neither `controlplane/` nor `dataplane/`; both may
> import `capability`.**

## Contents

| Package | Answers |
| --- | --- |
| [circuitbreaker](circuitbreaker/) | What is this route's/endpoint's circuit state? |
| [bulkhead](bulkhead/) | How many callers may hold a concurrency slot at once? |
| [ratelimit](ratelimit/) | How much budget is left, and how is it counted? |
| [fault](fault/) | Which rule governs this call, and what did it decide? |
| [traffic](traffic/) | Is this call real traffic or a load-test one? |
| [label](label/) | How is a governed service named (`prefix:name`)? |
| [discovery](discovery/) | Which live `host:port` addresses serve this name? |
| [loadbalance](loadbalance/) | Given the live set, which endpoint gets this request? |
| [matcher](matcher/) | Which rules does this string map match? |

`discovery` and `loadbalance` are consumed mostly by outbound code, but
their APIs carry no direction vocabulary — they resolve a name and select an
endpoint. `fault` and `loadbalance` also carry state that a control-plane root
drives. They belong here by concept: neither the direction nor the document
source is theirs.

## Notes

- The directory is a namespace: it holds no Go package of its own.
- OTel instrumentation scope strings inside these packages keep their
  historical paths (for example `.../dataplane/discovery`). They are
  backend-facing contracts, not import paths.
- If one of these packages later gains consumers outside the governance runtime
  (cache, lock, scheduler, config), the consumption-surface rule promotes it to
  `cloud/` rather than keeping it here.
