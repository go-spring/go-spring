# starter-config-k8s Example

Kubernetes ConfigMap config loading with starter-config-k8s.

## Features

- **ConfigMap loading**: Read config from K8s ConfigMap
- **Hot reload**: Watch ConfigMap changes and refresh dynamically

> Note: reading and watching a real ConfigMap needs a Kubernetes cluster. Without
> one the example still boots and self-terminates (the import is `optional:`),
> and `check.sh` exercises the provider against the client-go fake clientset.

## Manual Testing

Needs to run inside a K8s cluster.

Terminal 1, start the service and keep it running:
```bash
cd starter-config-k8s/example
go run . -manual
```

Running directly on a local machine prints a message and exits normally. Press `Ctrl+C` to stop after verification.

In another terminal, edit the ConfigMap and watch the field print:

```bash
kubectl edit configmap app-config      # or: kubectl patch configmap app-config ...
# prints: demo.message: "..." -> "manual-1"
```

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the unit tests (fake clientset, no cluster needed) and then boots
the example; outside a cluster the example self-terminates with exit code 0.