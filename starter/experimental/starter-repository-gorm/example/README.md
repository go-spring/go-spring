# starter-repository-gorm Example

Demonstrates the generic Repository pattern.

## Features

- **CRUD Operations**: Full Create/Read/Update/Delete workflow
- **Paginated Queries**: Supports pagination parameters
- **Compound Conditions**: Multi-field combined queries
- **Audit Fields**: Auto-fills creator/modifier

## Manual Testing

```bash
cd starter-repository-gorm/example
go run . -manual
```

The service keeps running. Press Ctrl+C to exit. Without `-manual`, `runTest()` executes automatically and exits.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.