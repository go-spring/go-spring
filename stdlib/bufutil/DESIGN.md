# bufutil Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. `bufutil` provides bounded buffers for
side-channel copying, where the copy must never backpressure or error the primary
flow.

## 1. Responsibilities & Boundaries

- Provide a `LimitedBuffer`: a `bytes.Buffer` with a byte cap. Writes past the cap
  are silently discarded, but `Write` always reports the full size and a nil
  error.
- Be the natural sink for an `io.TeeReader` that mirrors a body into a capture
  buffer (e.g. a request/response body captured for an access log): the body keeps
  flowing to the handler unchanged, while the capture is bounded so a runaway body
  can't exhaust memory.
- Not a general-purpose ring buffer, not a streaming parser, not a backpressure
  primitive. If you need flow control, this is the wrong type - it is deliberately
  lossy and error-free.

## 2. Key Seams

- **Silent overflow + full-write lie**: the defining property. `Write` drops bytes
  past the cap yet returns `len(p), nil`. This is what keeps an `io.TeeReader`
  unblocked when the capture is full - the alternative (erroring or short-writing)
  would corrupt the primary read.
- **`New(max)` constructor, zero-value = cap 0**: a zero-value `LimitedBuffer`
  discards everything, which is a safe default; callers must opt in to a non-zero
  cap. Negative caps panic (programmer error, not a runtime condition).
- **`Bytes()` aliases internal storage**: inherited from `bytes.Buffer`; the slice
  is invalidated by the next `Write`/`Reset`. Callers that need a stable copy take
  it before the next write.

## 3. Constraints

- Lossy by design. Data past the cap is gone with no record of how much was
  dropped beyond `Write`'s return (which always claims full success). Callers that
  need exact-byte accounting must track it themselves.
- Not safe for concurrent use, mirroring `bytes.Buffer`.
- The cap is set once at construction and never grows; `Reset` clears contents but
  keeps the cap.
