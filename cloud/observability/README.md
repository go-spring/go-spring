# observability

[English](README.md) | [中文](README_CN.md)

`cloud/observability` is **plane-neutral telemetry vocabulary**, not a plane and
not a capability SDK. It holds the pieces that both planes — and every
capability package — may use, but that none of them owns:

| Package | Answers |
| --- | --- |
| [attrs](attrs/) | Per-request attributes carried on a `context.Context` (so spans started on your behalf pick them up), plus the one shared duration-histogram bucket definition. |
| [instrument](instrument/) | The meter/tracer scopes, the access-log tag and the small renderers shared by the outbound and inbound observers. |
| [conf](conf/) | `conf.Refresh`: the shared instrumentation funnel for application-wide property refreshes (metrics + the one fleet-level log line). |

Neither imports `chain`, `guard`, `breaker` or `resilience`, so a cache, a lock,
a scheduler, a config starter or the control-plane registration package can use
them without pulling the governance stack into its dependency graph.

This directory is deliberately **not** under `governance/dataplane/`: the governed-call
instrumentation (which *is* data plane, and the outermost layer of it) lives in
each direction's tree —
[outbound/observe](../governance/dataplane/outbound/observe/) wraps
`chain.Executor`, and [inbound/observe](../governance/dataplane/inbound/observe/)
drives the admission authority's Admit/Done.

Dependency rule: neutral vocabulary at the `cloud/` top level — `propagate`,
`mesh`, `observability/attrs`, `observability/conf` — may be imported by both
planes and by any capability; the planes may import each other only in the
allowed direction (control plane → data plane), and never upward into a
capability SDK.
