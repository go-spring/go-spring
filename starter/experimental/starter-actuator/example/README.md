# starter-actuator Example

Health check, readiness probe, and startup probe endpoints for starter-actuator.

## Features

- **Health Check `/health`**: Always returns UP
- **Readiness Probe `/readiness`**: Aggregates custom HealthIndicators, reflecting dependency status
- **Startup Probe `/startup`**: Reflects startup completion status
- **Application Info `/info`**: Exposes application metadata
- **Auth**: the whole management port requires `Authorization: Bearer dev-actuator-token`
- **Default-off introspection**: `/loggers /env /configprops /threaddump` are enabled via
  `spring.actuator.endpoints.include`; `/beans` stays off (404)
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
# -> 401 unauthorized (no token)

curl -H 'Authorization: Bearer dev-actuator-token' http://127.0.0.1:9370/health
# -> {"status":"UP"}

curl -H 'Authorization: Bearer dev-actuator-token' http://127.0.0.1:9370/readiness
# -> {"status":"UP"}

curl -H 'Authorization: Bearer dev-actuator-token' http://127.0.0.1:9370/info
# -> {"app":{...}}

curl -H 'Authorization: Bearer dev-actuator-token' http://127.0.0.1:9370/env
# -> configuration sources, secrets masked

curl -H 'Authorization: Bearer dev-actuator-token' http://127.0.0.1:9370/beans
# -> 404 (sensitive endpoint not in endpoints.include)
```

Press Ctrl+C to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.