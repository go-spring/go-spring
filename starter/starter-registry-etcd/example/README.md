# starter-registry-etcd Example

Demonstrates etcd service registration and discovery of starter-registry-etcd.

## Features

- **Service Registration**: Register the service to etcd on startup
- **Service Discovery**: Query registered service keys via etcd API
- **Auto Deregistration**: Automatically deregister from etcd on process exit

> Requires etcd service running. `check.sh` starts etcd via docker compose.

## Manual Testing

```bash
cd starter-registry-etcd/example
go run . -manual
```

Expected output:
```
instances found: ...
```

Start etcd first:
```bash
# Start etcd
docker compose up -d

# Run the example
go run . -manual

# View registered keys
etcdctl get --prefix /go-spring/
```

## Smoke Test

```bash
./check.sh
```

`check.sh` starts etcd via docker compose, runs the example and verifies registration, exit code 0 means pass.