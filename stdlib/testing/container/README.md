# container

[English](README.md) | [中文](README_CN.md)

`container` is a Testcontainers-style helper for **slice tests**: it starts a real
dependency (redis, mysql, ...) in a Docker container from inside a Go test and
tears it down automatically when the test ends.

## How it differs from a starter's `check.sh`

A starter's `check.sh` is an out-of-process shell smoke test. This helper is for
in-process `go test`: [`Run`](container.go) blocks until the container is ready,
registers cleanup with `t.Cleanup`, and returns the **dynamically mapped**
host:port so the test dials the service directly. No fixed ports, no manual
`docker rm`.

## Zero dependencies, plain `docker`

The helper shells out to the `docker` CLI via `os/exec` instead of importing a
Docker SDK. This keeps `stdlib`'s zero-dependency rule and reuses the repository's
Docker convention (a plain `docker` binary, no compose-v2 plugin). Any proxy
needed to pull an image is inherited from the environment — export it before
`go test` exactly as the starter `check.sh` scripts do.

Always guard with `SkipIfNoDocker(t)` so tests skip cleanly where Docker is
unavailable.

## Usage

```go
func TestRedis(t *testing.T) {
    container.SkipIfNoDocker(t)

    addr := container.Redis(t) // redis:7, port-mapped, cleanup registered
    // ... dial addr and run assertions ...
}
```

Or drive an arbitrary image with `Run`:

```go
c := container.Run(t, container.Request{
    Image:        "mysql:8",
    Env:          map[string]string{"MYSQL_ROOT_PASSWORD": "secret"},
    ExposedPorts: []string{"3306"},
    WaitFor:      container.WaitForLog("ready for connections", 90*time.Second),
})
dsn := "root:secret@tcp(" + c.Endpoint("3306") + ")/"
```

### Readiness (`WaitFor`)

- `WaitForListeningPort(port, timeout)` — cheapest, most portable; waits until
  the mapped port accepts a TCP connection.
- `WaitForLog(substr, timeout)` — waits for a log line, for services that accept
  connections before they are truly ready (databases running init scripts).

### Handle

- `c.Endpoint(port)` — dialable `"host:port"` for an exposed container port.
- `c.MappedPort(port)` — just the host-side port number.
- `c.Host()` — the host (loopback).

### Presets

- `Redis(t)` — starts `redis:7`, returns its endpoint.
- `MySQL(t, rootPassword, database)` — starts `mysql:8`, returns its endpoint.

See [`container_test.go`](container_test.go) for the real-redis acceptance test.

## License

Apache License 2.0
