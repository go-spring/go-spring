# starter-config-file Example

Demonstrates local file config hot reload with starter-config-file.

## Features

- **Config loading**: Read initial values from `conf/app.properties`
- **File watching**: Watch file changes via `Dync[T]`, auto hot reload
- **Dynamic value verification**: Config values update in real time after file modification

## Manual Testing

```bash
cd starter-config-file/example
go run . -manual
```

The program keeps running, press Ctrl+C to exit. Without -manual, runTest() executes automatically and exits.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for its self-test to complete, exit code 0 means pass.
