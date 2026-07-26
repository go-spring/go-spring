# podinfo

[English](README.md) | [中文](README_CN.md)

## Overview

`podinfo` exposes Kubernetes Pod metadata — name, namespace, IP, node, service
account, and labels — to the application, with **zero third-party dependencies**.

It does **not** talk to the Kubernetes API server. Instead it relies on the
[Downward API](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/):
the Deployment injects Pod fields as environment variables and mounts
labels/annotations as a file. Go-Spring's config layer maps `GS_`-prefixed
environment variables into the property tree (`GS_POD_NAME` → `pod.name`), so the
`PodInfo` fields bind straight from configuration.

## Environment variable conventions

The `gs k8s` scaffolding generates a Deployment that wires these via the Downward
API. If you write the manifests by hand, match these names:

| Env var                   | Property               | Downward API source          |
|---------------------------|------------------------|------------------------------|
| `GS_POD_NAME`             | `pod.name`             | `metadata.name`              |
| `GS_POD_NAMESPACE`        | `pod.namespace`        | `metadata.namespace`         |
| `GS_POD_IP`               | `pod.ip`               | `status.podIP`               |
| `GS_NODE_NAME`            | `node.name`            | `spec.nodeName`              |
| `GS_POD_SERVICE_ACCOUNT`  | `pod.service.account`  | `spec.serviceAccountName`    |
| `pod.labels.path` (config)| `pod.labels.path`      | labels volume mount path     |

> Note: `pod.labels.path` is set in the `k8s` config profile (not an env var),
> because the `GS_` env → property mapping cannot produce hyphens or arbitrary
> paths. Labels are mounted as a file (e.g. `/etc/podinfo/labels`).

## Usage

`PodInfo` carries `value` tags but imports nothing from the IoC container, so it
stays in the zero-dependency stdlib layer. Register it as a bean and autowire it:

```go
gs.Object(&podinfo.PodInfo{})

type MyService struct {
    Pod *podinfo.PodInfo `autowire:""`
}

func (s *MyService) Describe() {
    fmt.Println(s.Pod.Name, s.Pod.Namespace, s.Pod.IP)
    labels, _ := s.Pod.Labels() // parses the mounted labels file
    fmt.Println(labels["app"])
}
```

Outside Kubernetes (no Downward API variables), every field is empty and
`Labels()` returns an empty map — the app wires and runs unchanged.

## API

- `PodInfo` — struct of Pod metadata fields, bound from configuration.
- `(*PodInfo) Labels() (map[string]string, error)` — reads and parses the
  Downward API labels file at `LabelsPath` (one `key="value"` per line). Returns
  an empty map when `LabelsPath` is unset.
- `(*PodInfo) Metadata() map[string]string` — the non-empty scalar fields as a
  map, suitable as a source of service-discovery registration metadata.
