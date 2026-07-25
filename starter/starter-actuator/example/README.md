# starter-actuator Example

Health check, readiness probe, and startup probe endpoints for starter-actuator.

## Features

- **Health Check `/health`**: Always returns UP
- **Readiness Probe `/readiness`**: Aggregates custom HealthIndicators, reflecting dependency status
- **Startup Probe `/startup`**: Reflects startup completion status
- **Application Info `/info`**: Exposes application metadata
- **Dynamic Toggle**: Toggle a HealthIndicator to verify readiness status changes

## Manual Testing

Terminal 1, start the service and keep it running:
```bash
cd starter-actuator/example
go run . -manual
```

Terminal 2, run verification commands:
```bash
curl http://127.0.0.1:9370/health
# -> {"status":"UP"}

curl http://127.0.0.1:9370/readiness
# -> {"status":"UP"}

curl http://127.0.0.1:9370/info
# -> {"app":{...}}
```

Press Ctrl+C to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.