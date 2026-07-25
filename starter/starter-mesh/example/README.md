# starter-mesh Example

Demonstrates the service mesh pattern with starter-mesh.

## Features

- **Mesh Mode Switching**: Compare load balancing behavior between mesh on/off modes
- **Pass-through Mode**: When mesh is on, degrades to pass-through, bypassing discovery and load balancing
- **Load Balancing**: When mesh is off, distributes evenly across multiple endpoints via discovery + LB

## Manual Testing

```bash
cd starter-mesh/example
go run .
```

Expected output:
```
OK: mesh mode degrades discovery + load balancing to a pass-through
```

## Smoke Test

```bash
./check.sh
```

check.sh runs the example and waits for self-test to complete, exit code 0 means pass.