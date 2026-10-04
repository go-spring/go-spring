# starter-config-apollo example

Self-contained: the example starts a mock Apollo config service (the meta,
configfiles, configs and notifications/v2 endpoints agollo drives), imports the
starter, and asserts the whole remote-config link — the property cold-loads into
a Dync field, and a publish to the mock hot-reloads it. No docker, no real
Apollo stack.

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
./check.sh   # the smoke test
```
