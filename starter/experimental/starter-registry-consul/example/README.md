# starter-registry-consul Example

Demonstrates Consul service registration and discovery.

## Features

- **Service Registration**: Registers the service to the Consul catalog on startup
- **Service Discovery**: Queries registered service instances via Consul API
- **Health Check**: Consul periodically checks service health status

> Requires Consul service running. `check.sh` starts Consul via docker compose.

## Manual Testing

```bash
cd starter-registry-consul/example
go run . -manual
```

Expected output:
```
instances found: ...
```

Start Consul first:
```bash
# Start Consul
docker compose up -d

# Run the example
go run . -manual

# View registered services
curl http://127.0.0.1:8500/v1/catalog/service/go-spring
```

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Consul via docker compose, runs the example and verifies registration, exit code 0 means pass.
