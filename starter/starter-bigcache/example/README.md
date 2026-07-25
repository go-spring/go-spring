# starter-bigcache Example

In-process hot cache for starter-bigcache.

## Features

- **SET/GET**: Write cache entries and read them back for verification
- **TTL Expiry**: Write entries with a short TTL; verify they return not-found after expiry
- **HTTP Endpoint**: Read cache values via the `/get` endpoint

## Manual Testing

Terminal 1, start the service and keep it running:
```bash
cd starter-bigcache/example
go run . -manual
```

Terminal 2, run verification commands:
```bash
curl http://127.0.0.1:9090/get
```

Press Ctrl+C to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.