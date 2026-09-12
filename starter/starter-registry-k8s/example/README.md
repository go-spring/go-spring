# starter-registry-k8s Example

Demonstrates Kubernetes service discovery with starter-registry-k8s.

## Features

- **Service Discovery**: Discover Service instances in the cluster via the K8s API
- **Backend Wiring**: Declare the K8s discovery backend bean (`k8s.main`) and resolve a Service through it

> Note: This example requires running inside a Kubernetes cluster. `check.sh` skips when no cluster is available.

## Manual Testing

Must run inside a K8s cluster.

Terminal 1, start the service and keep it running:
```bash
cd starter-registry-k8s/example
go run . -manual
```

Running locally prints a message and exits normally. Press `Ctrl+C` to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example, automatically skips when no K8s cluster is available.