# starter-batch Example

Demonstrates batch job execution and checkpoint-based resume for starter-batch.

## Features

- **Two-Phase Execution**: Phase 1 simulates a checkpoint crash, Phase 2 resumes from checkpoint
- **Checkpoint Resume**: After a crash, restart continues from the last checkpoint
- **Job Registration**: Register batch jobs via the `JobDefinition` interface

## Manual Testing

```bash
cd starter-batch/example
go run . -manual
```

Expected output (two phases execute automatically, Phase 2 resumes from checkpoint and completes):
```
phase 1: ...
phase 2: ...
```

The service keeps running, can verify with corresponding CLI tools. Press `Ctrl+C` to exit.

Requires Docker environment to run full smoke test (depends on MySQL).

## Smoke Test

```bash
./check.sh
```

`check.sh` starts MySQL via docker compose, runs the example and verifies checkpoint resume, exit code 0 means pass.