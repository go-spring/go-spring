# starter-config-file Example

Local file config hot reload with starter-config-file.

## Features

- **Config loading**: Read initial values from `conf/app.properties`
- **File watching**: Watch file changes via `Dync[T]`; hot reloads automatically
- **Dynamic value verification**: Config values update in real time after file modification

## Manual Testing

```bash
cd starter-config-file/example
go run . -manual
```

The program keeps running. Press Ctrl+C to exit. Without `-manual`, `runTest()` runs automatically and exits.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for its self-test to complete, exit code 0 means pass.
