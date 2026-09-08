# podinfo

[English](README.md) | [中文](README_CN.md)

`podinfo` exposes Kubernetes Pod metadata — name, namespace, UID, IP, node, host
IP, service account, and labels — to the application, with **zero third-party
dependencies**.

It does **not** talk to the Kubernetes API server: no client-go, no informers,
no watches. Every field comes from configuration or a mounted file via the
[Downward API](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/):
the Deployment injects Pod fields as environment variables and mounts
labels/annotations as a file. Go-Spring's config layer maps `GS_`-prefixed
environment variables into the property tree (`GS_POD_NAME` → `pod.name`), so the
`PodInfo` fields bind straight from configuration.

## Installation

```
go get go-spring.org/cloud
```

## Environment variable conventions

The `gs k8s` scaffolding generates a Deployment that wires these via the Downward
API. If you write the manifests by hand, match these names:

| Env var                   | Property               | Downward API source          |
|---------------------------|------------------------|------------------------------|
| `GS_POD_NAME`             | `pod.name`             | `metadata.name`              |
| `GS_POD_NAMESPACE`        | `pod.namespace`        | `metadata.namespace`         |
| `GS_POD_UID`              | `pod.uid`              | `metadata.uid`               |
| `GS_POD_IP`               | `pod.ip`               | `status.podIP`               |
| `GS_NODE_NAME`            | `node.name`            | `spec.nodeName`              |
| `GS_NODE_IP`              | `node.ip`              | `status.hostIP`              |
| `GS_POD_SERVICE_ACCOUNT`  | `pod.service.account`  | `spec.serviceAccountName`    |
| `pod.labels.path` (config) | `pod.labels.path`      | labels volume mount path     |

> Note: `pod.labels.path` is set in the `k8s` config profile (not an env var),
> because the `GS_` env → property mapping cannot produce hyphens or arbitrary
> paths. Labels are mounted as a file (e.g. `/etc/podinfo/labels`).

## Usage

`PodInfo` carries `value` tags but imports nothing from the IoC container, so it
cannot register itself. Register it as a bean — one line in an init file — and
autowire the pointer wherever the Pod facts are needed:

```go
func init() { gs.Object(&podinfo.PodInfo{}) }

type MyService struct {
    Pod *podinfo.PodInfo `autowire:""`
}

func (s *MyService) Describe() {
    fmt.Println(s.Pod.Name, s.Pod.Namespace, s.Pod.IP)
    labels, _ := s.Pod.Labels() // parses the mounted labels file
    fmt.Println(labels["app"])
}
```

Who provides the bean: **your application** — the `gs.Object(&podinfo.PodInfo{})`
line above. The framework does not provide it; writing this line is the
application's job.

Outside Kubernetes (no Downward API variables), every field is empty and
`Labels()` returns an empty map — the app wires and runs unchanged. That is also
why the `value` tags default to empty instead of failing wiring: the same code
runs on a laptop and in a cluster.
