# starter-lock-k8s Example

Run this example to test Kubernetes Lease distributed lock with `starter-lock-k8s`.

## Features

- **Leader Election**: Compete for leadership via K8s Lease resource
- **Lock Acquisition/Release**: Execute business logic after acquiring the lock, then release it

> Note: This example must run inside a Kubernetes cluster. `check.sh` skips when no cluster is available.

## Manual Testing

Must run inside a K8s cluster.

Terminal 1, start the service and keep it running:
```bash
cd starter-lock-k8s/example
go run . -manual
```

Running directly locally prints a notice and exits gracefully. Press `Ctrl+C` to stop after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example, automatically skips when no K8s cluster is available.
