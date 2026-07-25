# starter-config-k8s Example

Demonstrates Kubernetes ConfigMap config loading with starter-config-k8s.

## Features

- **ConfigMap loading**: Read config from K8s ConfigMap
- **Hot reload**: Watch ConfigMap changes and refresh dynamically

> Note: This example needs to run inside a Kubernetes cluster. `check.sh` will skip when no cluster environment is available.

## Manual Testing

Needs to run inside a K8s cluster.

Terminal 1, start the service and keep it running:
```bash
cd starter-config-k8s/example
go run . -manual
```

Running directly on local machine will print a message and exit normally. Press `Ctrl+C` to stop after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example, automatically skips when no K8s cluster is available.