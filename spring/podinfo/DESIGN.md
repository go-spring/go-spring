# podinfo Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`podinfo` is the zero-dependency stdlib helper that exposes Kubernetes Pod
metadata to an application. It is one leg of the Task 05 K8s deployment
scaffolding (the other legs are the `gs k8s` code generator and the k8s config
profile in `layout`).

## 1. Responsibilities and Boundaries

- Bind Pod metadata (name, namespace, IP, node, service account, labels path)
  from configuration properties, so an application reads Pod facts as regular
  autowired fields.
- Parse the Downward API labels file when its mount path is known.
- Refuse to talk to the Kubernetes API server. Every field is populated from
  configuration (Downward API env variables) or a mounted file. No client-go,
  no informers, no watches.

## 2. Key Abstractions and Seams

- **`PodInfo` struct with `value` tags** — `${pod.name:=}` and friends. Empty
  defaults let the same code run outside Kubernetes with zero values instead of
  wiring failures.
- **`Labels()`** — reads and unquotes each `key="value"` line in the Downward
  API labels file. A malformed line degrades to the raw value; a missing file
  path (unset `pod.labels.path`) returns an empty map with no error.
- **`Metadata()`** — the subset of scalar fields suitable as service-discovery
  registration metadata. `LabelsPath` is intentionally excluded: it is an
  implementation detail, not something to publish.

## 3. Constraints

- `spring/podinfo` is a **subpackage of `go-spring.org/stdlib`**, not its own
  module. It has no separate `go.mod` and it does not appear in `go.work`.
- The struct carries `value` tags but never imports the IoC container, so it
  stays in the zero-dependency layer. Callers register it with
  `gs.Object(&podinfo.PodInfo{})`.
- Env variable names are pinned by convention: `GS_POD_NAME`,
  `GS_POD_NAMESPACE`, `GS_POD_IP`, `GS_NODE_NAME`, `GS_POD_SERVICE_ACCOUNT`.
  `pod.labels.path` is configured **in a profile file** (usually
  `app-k8s.properties`) because the `GS_`-env-to-property mapping cannot emit
  hyphens or arbitrary paths — do not add it as an env variable.

## 4. Trade-offs and Alternatives Rejected

- **No Kubernetes client dependency.** Adding client-go would break the
  zero-dependency rule and pull in a large transitive tree; the Downward API
  covers everything a workload actually needs.
- **No auto-registration.** `stdlib` cannot import `gs`, so `podinfo` will not
  register itself. Applications call `gs.Object(&podinfo.PodInfo{})` — a single
  line in an init file.
