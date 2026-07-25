# starter-registry-zookeeper Example

Demonstrates ZooKeeper service registration and discovery of starter-registry-zookeeper.

## Features

- **Service Registration**: Register the service to ZooKeeper on startup
- **Service Discovery**: Query registered znodes via ZK API
- **Auto Deregistration**: Automatically deregister from ZK on process exit

> Requires ZooKeeper service running. `check.sh` starts ZooKeeper via docker compose.

## Manual Testing

```bash
cd starter-registry-zookeeper/example
go run . -manual
```

Expected output:
```
instances found: ...
```

Start ZooKeeper first:
```bash
# Start ZooKeeper
docker compose up -d

# Run the example
go run . -manual
```

## Smoke Test

```bash
./check.sh
```

`check.sh` starts ZooKeeper via docker compose, runs the example and verifies registration, exit code 0 means pass.