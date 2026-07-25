# starter-casbin Example

RBAC access control for starter-casbin.

## Features

- **Policy File Loading**: Load RBAC policies from `model.conf` and `policy.csv`
- **Permission Enforcement**: Check whether subject/object/action is allowed via the `/enforce` endpoint
- **Policy Hot Reload**: Watch policy file changes via Watcher

## Manual Testing

Terminal 1, start the service and keep it running:
```bash
cd starter-casbin/example
go run . -manual
```

Terminal 2, run verification commands:
```bash
curl 'http://127.0.0.1:9090/enforce?sub=alice&obj=/data&act=write'
# -> allow

curl 'http://127.0.0.1:9090/enforce?sub=bob&obj=/data&act=write'
# -> deny
```

Press Ctrl+C to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.