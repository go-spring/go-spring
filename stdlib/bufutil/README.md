# bufutil

[English](README.md) | [中文](README_CN.md)

`bufutil` provides bounded buffers for side-channel copying, where the copy must
never backpressure or error the primary flow.

A `LimitedBuffer` wraps a `bytes.Buffer` with a byte cap: writes past the cap are
silently discarded, but `Write` always reports the full size and a nil error. That
makes it the natural sink for an `io.TeeReader` that mirrors a request or response
body into a capture buffer for logging - the body keeps flowing to the handler
unchanged, while the capture is bounded so a runaway body can't exhaust memory.

This is deliberately unlike a plain `bytes.Buffer` (which grows unboundedly) and
unlike `io.LimitReader` (which only limits reads and errors on overflow): a
`LimitedBuffer` is a write-side, error-free bound.

## Usage

Capture a body via `io.TeeReader` without risking memory exhaustion:

```go
import (
    "io"
    "go-spring.org/stdlib/bufutil"
)

// Mirror the request body into a bounded capture for the access log. The handler
// still reads every byte; the capture keeps at most 512 KiB.
capture := bufutil.New(512 * 1024)
body = io.TeeReader(r.Body, capture)
// ... handler reads `body` ...
log.Printf("req.body=%s", capture.String())
```

### API

| Method | Description |
|---|---|
| `New(max int) *LimitedBuffer` | Create a buffer that keeps at most `max` bytes (panics if `max < 0`). |
| `Write(p []byte) (int, error)` | Append up to the cap, drop overflow; always returns `len(p), nil`. |
| `WriteString(s string) (int, error)` | String convenience over `Write`. |
| `Bytes() []byte` | Buffered bytes (aliases internal storage). |
| `String() string` | Buffered bytes as a string. |
| `Len() int` / `Cap() int` | Current / max size. |
| `Reset()` | Discard contents (keeps the cap) for reuse. |

See [DESIGN.md](DESIGN.md) for the rationale behind the lossy, error-free contract.
