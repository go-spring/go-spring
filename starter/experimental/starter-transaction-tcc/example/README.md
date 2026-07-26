# starter-transaction-tcc Example

Demonstrates TCC distributed transactions (Try-Confirm-Cancel mode) using starter-transaction-tcc.

## Features

- **Commit path**: Try → Confirm, data consistent across both services
- **Rollback path**: Try → Cancel, data restored to pre-transaction state
- **Transaction coordination**: Coordinator manages each TCC phase

## Manual Testing

```bash
cd starter-transaction-tcc/example
go run . -manual
```

The program keeps running, press Ctrl+C to exit. Without -manual, runTest() executes automatically and exits.

## Smoke Test

```bash
./check.sh
```

check.sh runs the example and waits for self-test to complete, exit code 0 means pass.
