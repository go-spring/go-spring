# Decision: why `starter-admin-ui` is intentionally minimal

## Context

Spring Boot Admin (SBA) is a popular tool in the Java ecosystem that
aggregates Spring Boot Actuator data from every registered application and
renders a rich management UI: live status, JVM metrics, thread dumps, log
level toggles, session inspection, notifications, and so on. On its face,
"Spring Boot Admin equivalent" is a natural item on the Go-Spring feature
backlog (it appeared as Task 13.11 in `temp/task-13-p3-devexp.md`).

Two things pull against a feature-for-feature port:

1. **K8s + Prometheus + Grafana already covers most of it.** For any
   production Go-Spring deployment on Kubernetes, `contrib/observability/`
   already gives you a fuller solution than SBA: time-series history,
   alerting, per-pod dashboards, log/trace correlation. The Grafana
   dashboards are the equivalent effect — with more depth — of the SBA
   overview screens. This is the officially recommended path.
2. **The Go-Spring project convention discourages framework-shape porting.**
   The user memory captured under `feedback_capability_equivalence.md`
   states: when filling a Java/Spring gap, describe the fix as
   "equivalent effect via idiomatic Go", not as a reimplementation of the
   original framework or mechanism.

## Decision

Ship `starter-admin-ui` as a **narrow, self-contained fallback** rather than
an SBA clone.

- **Scope:** poll the four actuator endpoints (`/health`, `/readiness`,
  `/startup`, `/info`) of a configured list of instances and render an
  aggregated HTML table plus a JSON snapshot API. Nothing else.
- **Explicitly out of scope:** JVM-style deep introspection (Go doesn't
  expose these through actuator), log-level toggles, alerting, history,
  auth/RBAC, notifications, cluster-wide management actions. These are
  either Grafana/Prometheus territory or better done through dedicated
  tooling.
- **Dependencies:** zero third-party imports. Go stdlib plus
  `go-spring.org/spring/gs` and `go-spring.org/stdlib/errutil` only. The
  HTML template is a Go string constant; no CDN, no external assets. This
  keeps the starter usable in air-gapped and minimal environments — the
  main niche where SBA-style tools shine and Prometheus is often absent.

## When to use which

| Situation | Recommended path |
| --- | --- |
| Kubernetes + Prometheus available | `contrib/observability/` (Grafana). |
| Kubernetes without a metrics stack, quick bring-up | `starter-admin-ui`. |
| On-prem / air-gapped, no Prometheus | `starter-admin-ui`. |
| Rich alerting, history, deep drilldown | Prometheus + Grafana + Loki/Tempo. |
| Log-level toggles, thread dumps | Not provided by this starter (Go idioms differ; use `pprof` + structured logging). |

## Consequences

- The starter can be maintained in a single file plus a config, without
  taking on a UI framework, a JS toolchain, or a persistence layer.
- Operators who need more than a live status table are pointed at the
  observability stack instead of asking this starter to grow.
- If a real need for richer per-instance introspection emerges, it can be
  addressed by extending `starter-actuator` (more endpoint contributions
  through `spring/endpoint`) rather than by expanding this dashboard.
