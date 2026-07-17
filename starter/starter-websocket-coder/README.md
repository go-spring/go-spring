# starter-websocket-coder

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-websocket-coder` contributes a configured `*websocket.AcceptOptions`
built on [coder/websocket](https://github.com/coder/websocket) to a Go-Spring
application. It does **not** own an HTTP server or a listening port: a WebSocket
connection is just an HTTP `Upgrade`, so you mount your WebSocket routes onto
whatever HTTP server the application already runs (net/http, gin, echo,
hertz, ...).

This is the coder/websocket sibling of `starter-websocket` (which uses
gorilla/websocket). Unlike gorilla, coder/websocket has no `Upgrader` object;
the server upgrade is the free function `websocket.Accept(w, r, *AcceptOptions)`.
This starter therefore contributes the `*websocket.AcceptOptions` itself as the
injectable bean.

## Installation

```bash
go get go-spring.org/starter-websocket-coder
```

## Quick Start

### 1. Import the `starter-websocket-coder` Package

Refer to the [example.go](example/example.go) file. A blank import is enough —
you only need its `init()` to register the `*websocket.AcceptOptions` provider:

```go
import _ "go-spring.org/starter-websocket-coder"
```

### 2. Tune the Accept Options (optional)

Add configuration in your project's [configuration file](example/conf/app.properties).
There is no server address here — the options are applied when upgrading on an
existing HTTP server, which owns the port and timeouts:

```properties
spring.websocket.insecureSkipVerify=false
spring.websocket.originPatterns=chat.example.com
spring.websocket.compressionMode=0
spring.websocket.compressionThreshold=0
```

To customize beyond these fields (e.g. `Subprotocols`), provide your own
`*websocket.AcceptOptions` bean — `OnMissingBean` lets it take precedence.

### 3. Mount WebSocket Routes on an HTTP Server

Inject the `*websocket.AcceptOptions` wherever you register HTTP routes and call
`websocket.Accept`. The example mounts routes on the built-in gs HTTP server by
providing a custom `*gs.HttpServeMux`:

```go
gs.Provide(func(c *Controller, opts *websocket.AcceptOptions) *gs.HttpServeMux {
    mux := http.NewServeMux()
    mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
        conn, _ := websocket.Accept(w, r, opts)
        c.Echo(r.Context(), conn)
    })
    return &gs.HttpServeMux{Handler: mux}
})
```

Because WebSocket connections are long-lived, disable the HTTP server's
read/write timeouts so they are not cut mid-stream:

```properties
spring.http.server.readTimeout=0
spring.http.server.writeTimeout=0
spring.http.server.idleTimeout=0
```

## Core Features

The [example](example/example.go) demonstrates three end-to-end features, each
asserted by the in-process `runTest` client:

* **Text echo (`/echo`)** — upgrades the request and echoes each text frame
  back to the caller via `conn.Read` / `conn.Write`.
* **JSON echo (`/json`)** — uses `wsjson.Read` / `wsjson.Write` to accept
  `{"name": "..."}` and reply with `{"message": "Hi, ..."}`.
* **HTTP middleware guard (`/guard`, applied to `/echo` and `/json` too)** —
  a `requireApp` middleware wraps the handlers and returns `403 Forbidden`
  before the WebSocket handshake unless the request carries
  `X-App: go-spring`. `runTest` asserts both the allowed and rejected paths.
