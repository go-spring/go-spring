# starter-migration-gorm Example

Run this example to test database migration with `starter-migration-gorm`.

## Features

- **Startup Migration**: Automatically execute pending migration scripts on application startup
- **Idempotency**: Second run does not re-execute already completed migrations
- **Checksum Protection**: Tampering with executed migration scripts causes startup failure (checksum mismatch)

## Manual Testing

```bash
cd starter-migration-gorm/example
go run . -manual
```

The program keeps running, press Ctrl+C to exit. Without `-manual`, `runTest()` executes automatically and exits.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
