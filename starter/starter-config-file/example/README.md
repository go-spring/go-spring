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

The service keeps running. Press `Ctrl+C` to stop.

In another terminal, rewrite the mounted file and watch it print:

```bash
echo 'demo.message=manual-1' > mount/application.properties
# prints: demo.message: "..." -> "manual-1"
```

Without `-manual` the example rewrites the mount itself (the kubelet's atomic
`..data` swap), waits for the bound field to hot-reload, prints
`hot-reload observed: updated-<hhmmss>`, and exits:

```bash
go run .
```

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for its self-test to complete, exit code 0 means pass.
