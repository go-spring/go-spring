# starter-websocket

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-websocket` provides a lightweight WebSocket server built on
[gorilla/websocket](https://github.com/gorilla/websocket), making it easy to
expose WebSocket routes from a Go-Spring application.

## Installation

```bash
go get go-spring.org/starter-websocket
```

## Quick Start

### 1. Import the `starter-websocket` Package

Refer to the [example.go](example/example.go) file.

```go
import StarterWebsocket "go-spring.org/starter-websocket"
```

### 2. Configure the WebSocket Server

Add configuration in your project's [configuration file](example/conf/app.properties).
The starter listens on `:9696` by default; disable the built-in HTTP server so
the two do not race for a port:

```properties
spring.http.server.enabled=false
```

### 3. Register WebSocket Routes

Provide a `StarterWebsocket.ServerRegister` bean; the starter passes the shared
`*http.ServeMux` and `*websocket.Upgrader` into it so you can wire routes:

```go
gs.Provide(func(c *Controller) StarterWebsocket.ServerRegister {
    return func(mux *http.ServeMux, upgrader *websocket.Upgrader) {
        mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
            conn, _ := upgrader.Upgrade(w, r, nil)
            c.Echo(conn)
        })
    }
})
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
