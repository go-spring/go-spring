# kernel

[English](README.md) | [中文](README_CN.md)

`kernel` is the **direction-agnostic half of the data plane**: the concepts and
mechanisms both call directions use, but that belong to neither.

## The rule

> A package belongs in `kernel` when it does not know which direction it is
> serving. It answers questions about addresses, endpoints, failure state,
> budget, traffic class or service naming — never about an outbound attempt or
> an inbound request.

Once code starts speaking `Executor`, `Operation`, `Recorder`, `Request`,
`Admit` or `Done`, it knows its direction and lives under
[outbound](../outbound/) or [inbound](../inbound/) instead.

This is also a dependency invariant:

> **`kernel` never imports `inbound/` or `outbound/`; the direction trees may
> import `kernel`.**

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

`discovery` and `loadbalance` are consumed only by outbound code today, but
their APIs carry no direction vocabulary — they resolve a name and select an
endpoint. They belong here by concept, not by current caller.

## Notes

- The directory is a namespace: it holds no Go package of its own.
- OTel instrumentation scope strings inside these packages keep their
  historical paths (for example `.../dataplane/discovery`). They are
  backend-facing contracts, not import paths.
- If one of these packages later gains consumers outside the request path
  (cache, lock, scheduler, config), the consumption-surface rule promotes it to
  `cloud/` rather than keeping it here.
