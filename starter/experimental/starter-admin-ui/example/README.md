# starter-admin-ui Example

Management interface for starter-admin-ui.

## Features

- **Management UI**: Web UI to view application status and configuration

## Manual Testing

Terminal 1, start the service and keep it running:
```bash
cd starter-admin-ui/example
go run . -manual
```

Terminal 2, open `http://127.0.0.1:9280` in browser to view the management interface.

Press Ctrl+C to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.