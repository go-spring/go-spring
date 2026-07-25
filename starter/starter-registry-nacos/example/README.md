# starter-registry-nacos Example

Demonstrates Nacos service registration and discovery of starter-registry-nacos.

## Features

- **Service Registration**: Register the service to Nacos on startup
- **Service Discovery**: Query registered service instances via Nacos Open API
- **Auto Deregistration**: Automatically deregister from Nacos on process exit

> Requires Nacos service running. `check.sh` starts Nacos via docker compose.

## Manual Testing

```bash
cd starter-registry-nacos/example
go run . -manual
```

Expected output:
```
instances found: ...
```

Start Nacos first:
```bash
# Start Nacos
docker compose up -d

# Run the example
go run . -manual

# View registered services
curl 'http://127.0.0.1:8848/nacos/v1/ns/instance/list?serviceName=go-spring'
```

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Nacos via docker compose, runs the example and verifies registration, exit code 0 means pass.