# starter-config-apollo Example

Apollo config management with starter-config-apollo.

## Features

- **Config loading**: Read config from the Apollo namespace
- **Config hot reload**: Publish a new value to the mock; the app picks it up in real time
- **Dync dynamic binding**: Bind config via `Dync[T]`; refreshes automatically

> Self-contained: the example starts an in-process mock Apollo service (the meta,
> configfiles, configs and notifications/v2 endpoints agollo drives), so no
> docker and no real Apollo stack are needed.

## Manual Testing

```bash
cd starter-config-apollo/example
go run . -manual
```

The mock Apollo service (bound to `127.0.0.1:18080`) starts in-process and the
app keeps running. Press `Ctrl+C` to stop.

In another terminal, publish a new value and watch it print:

```bash
curl -fsS -X POST 'http://127.0.0.1:18080/publish?value=manual-1'
# prints: demo.message: "..." -> "manual-1"
```

Without `-manual` the example asserts the cold-loaded value, publishes a new one
itself, waits for the hot-reload, prints `hot-reload observed: hello-<hhmmss>`,
and exits.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example (it starts the mock itself), which cold-loads the
value, publishes a new one and verifies the refresh, exit code 0 means pass.
