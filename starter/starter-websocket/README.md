# starter-websocket

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-websocket` contributes a configured `*websocket.Upgrader` built on
[gorilla/websocket](https://github.com/gorilla/websocket) to a Go-Spring
application. It does **not** own an HTTP server or a listening port: a WebSocket
connection is just an HTTP `Upgrade`, so you mount your WebSocket routes onto
whatever HTTP server the application already runs (net/http, gin, echo,
hertz, ...).

## Installation

```bash
go get go-spring.org/starter-websocket
```

## Quick Start

### 1. Import the `starter-websocket` Package

Refer to the [example.go](example/example.go) file. A blank import is enough —
you only need its `init()` to register the `*websocket.Upgrader` provider:

```go
import _ "go-spring.org/starter-websocket"
```

### 2. Tune the Upgrader (optional)

Add configuration in your project's [configuration file](example/conf/app.properties).
There is no server address here — the upgrader is mounted on an existing HTTP
server, which owns the port and timeouts:

```properties
spring.websocket.handshakeTimeout=10s
spring.websocket.readBufferSize=1024
spring.websocket.writeBufferSize=1024
# Enable permessage-deflate compression.
spring.websocket.enableCompression=false
# Origin allowlist matched against the request's Origin header. Empty keeps
# gorilla's default same-origin policy; a single "*" accepts any origin.
spring.websocket.allowedOrigins=
```

To customize beyond these fields (e.g. `CheckOrigin` or compression), provide
your own `*websocket.Upgrader` bean — `OnMissingBean` lets it take precedence.

### 3. Mount WebSocket Routes on an HTTP Server

Inject the `*websocket.Upgrader` wherever you register HTTP routes and call
`Upgrade`. The example mounts routes on the built-in gs HTTP server by providing
a custom `*gs.HttpServeMux`:

```go
gs.Provide(func(c *Controller, upgrader *websocket.Upgrader) *gs.HttpServeMux {
    mux := http.NewServeMux()
    mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
        conn, _ := upgrader.Upgrade(w, r, nil)
        c.Echo(conn)
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
  back to the caller via `conn.ReadMessage` / `conn.WriteMessage`.
* **JSON echo (`/json`)** — uses `conn.ReadJSON` / `conn.WriteJSON` to accept
  `{"name": "..."}` and reply with `{"message": "Hi, ..."}`.
* **HTTP middleware guard (`/guard`, applied to `/echo` and `/json` too)** —
  a `requireApp` middleware wraps the handlers and returns `403 Forbidden`
  before the WebSocket handshake unless the request carries
  `X-App: go-spring`. `runTest` asserts both the allowed and rejected paths.
