# starter-pprof - pprof Profiling Integration

> pprof is Go's built-in profiling tool, and this starter provides convenient integration.

## Installation

```go
import _ "github.com/go-spring/starter-pprof"
```

## Configuration

```properties
# Whether to enable it (default: true)
pprof.enable=true

# Listen address (default: :6060)
pprof.addr=:6060
```

## Usage

After starting the application, you can profile it in the following ways:

### Command Line

```bash
# View CPU overview
go tool pprof http://localhost:6060/debug/pprof/profile

# View heap memory
go tool pprof http://localhost:6060/debug/pprof/heap

# View goroutines
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### Web UI

Visit `http://localhost:6060/debug/pprof/` in a browser to view interactive flame graphs.
