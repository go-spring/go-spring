# starter-swagger Example

Demonstrates Swagger UI integration.

## Features

- **Swagger UI**: Provides an interactive API documentation interface
- **OpenAPI Spec**: Auto-generates OpenAPI specification JSON
- **API Testing**: Test API endpoints directly through the UI

## Manual Testing

Terminal 1, start the service:
```bash
cd starter-swagger/example
go run . -manual
```

Terminal 2, run verification commands:
```bash
curl http://127.0.0.1:9090/swagger/index.html
# -> Swagger UI HTML

curl http://127.0.0.1:9090/swagger/doc.json
# -> OpenAPI spec JSON
```

Press Ctrl+C to stop the service.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.