# starter-config-k8s Example

Kubernetes ConfigMap config loading with starter-config-k8s.

## Features

- **ConfigMap loading**: Read config from K8s ConfigMap
- **Hot reload**: Watch ConfigMap changes and refresh dynamically

> Note: reading and watching a real ConfigMap needs a Kubernetes cluster. Without
> one the example still boots and self-terminates (the import is `optional:`).

## Manual Testing

```bash
cd starter-config-k8s/example
go run . -manual
```

Must be run inside a K8s cluster (see `deploy/`). Running directly on a local
machine prints a message and exits normally.

The service keeps running. Press `Ctrl+C` to stop.

In another terminal, edit the ConfigMap and watch the field print:

```bash
kubectl edit configmap app-config      # or: kubectl patch configmap app-config ...
# prints: demo.message: "..." -> "manual-1"
```

## Smoke Test

```bash
./check.sh
```

`check.sh` boots the example; outside a cluster the read is skipped, the field
shows its default and the example self-terminates, exit code 0 means pass.