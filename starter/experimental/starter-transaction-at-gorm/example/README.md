# starter-transaction-at-gorm Example

Demonstrates AT distributed transactions (Seata AT mode) using starter-transaction-at-gorm.

## Features

- **Commit path**: Normal transaction commit, two databases remain consistent
- **Rollback path**: Automatic rollback on transaction failure, data restored to pre-transaction state
- **Write-write isolation**: Transaction isolation guarantees for concurrent write operations

## Manual Testing

```bash
cd starter-transaction-at-gorm/example
go run . -manual
```

The program keeps running, press Ctrl+C to exit. Without -manual, runTest() executes automatically and exits.

## Smoke Test

```bash
./check.sh
```

check.sh runs the example and waits for self-test to complete, exit code 0 means pass.
