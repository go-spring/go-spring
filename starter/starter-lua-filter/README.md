# starter-lua-filter

[English](README.md) | [中文](README_CN.md)

`starter-lua-filter` lets you write HTTP request filters in Lua, powered by
[gopher-lua](https://github.com/yuin/gopher-lua). Each filter runs at the
`net/http` layer — the same position an Envoy or OpenResty Lua filter occupies
in a gateway data plane — so it stays agnostic to whichever web framework
(gin/echo/hertz/net-http) serves the routes behind it. A filter can observe the
request, mutate response headers, or short-circuit with a deny, all without
recompiling the Go binary.

## Installation

```bash
go get go-spring.org/starter-lua-filter
```

## Quick Start

### 1. Import the `starter-lua-filter` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-lua-filter"
```

### 2. Configure the Lua Filter

Add filter configuration in your project's [configuration file](example/conf/app.properties).
Each entry points at a Lua script file (resolved relative to the working directory):

```properties
spring.lua.filter.guard.script=./scripts/guard.lua
```

### 3. Wire the Filter into the HTTP Server

The filter is injected by its config sub-key (`guard`) and wraps your business
handler. Handing the wrapped handler to a `*gs.HttpServeMux` places the filter
in front of the server. Refer to the [example.go](example/example.go) file.

```go
gs.Provide(func(guard *StarterLuaFilter.Filter) *gs.HttpServeMux {
    mux := http.NewServeMux()
    mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
        _, _ = w.Write([]byte("hello"))
    })
    return &gs.HttpServeMux{Handler: guard.Wrap(mux)}
}, gs.TagArg("guard"))
```

### 4. Write the Lua Script

Refer to the [guard.lua](example/scripts/guard.lua) file. The script runs once
per request against the host API below:

```lua
log("incoming " .. req.method .. " " .. req.path)
resp.set_header("X-Lua-Filter", "guard")
if req.path == "/admin" and req.header("X-Token") ~= "sesame" then
    deny(403, "forbidden: bad token")
    return
end
```

## Host API

The script sees these globals, re-bound per request:

| Symbol | Description |
| --- | --- |
| `req.method` / `req.path` | request method and URL path (strings) |
| `req.header(name)` | read a request header, `""` if absent |
| `req.query(name)` | read a URL query parameter, `""` if absent |
| `resp.set_header(name, value)` | set a response header |
| `deny(status, message)` | write `status`+`message` and short-circuit the chain; `return` right after |
| `log(message)` | log through the go-spring log pipeline |

## Core Features

The [example.go](example/example.go) program demonstrates and asserts three
core filter behaviors:

* **Pass-through + mutate** — a normal `/hello` request reaches the handler and
  carries the `X-Lua-Filter` header the script injected.
* **Gate** — `/admin` without the token is short-circuited with `403`; the
  business handler is never reached.
* **Conditional pass** — the same `/admin` path succeeds once the script's token
  condition is satisfied.

## Advanced Features

* **Multiple filters**: define several entries under `spring.lua.filter.*` and
  select each by name with `gs.TagArg("...")`.
* **Framework-agnostic**: because the filter wraps a plain `http.Handler`, the
  same script works whether gin, echo, hertz, or net/http serves the routes.
* **Sandboxed VM**: each request borrows a pooled `*lua.LState` that opens only
  the `base`/`table`/`string`/`math` libraries — filesystem and loader escapes
  (`dofile`/`loadfile`/`load`) are stripped.
