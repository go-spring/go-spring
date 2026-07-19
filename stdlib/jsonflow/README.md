# jsonflow
[English](README.md) | [中文](README_CN.md)

`jsonflow` is Go-Spring's streaming JSON layer. It sits on top of Go 1.26's
`encoding/json/v2` + `encoding/json/jsontext` and provides:

- A drop-in `Marshal` / `Unmarshal` / `MarshalWrite` / `UnmarshalRead` API
  with sensible defaults (deterministic map key order, nil map / slice as
  `null`).
- A generic set of `Encode<T>` / `Decode<T>` helpers for hand-written
  streaming encoders / decoders, generated code, and custom
  `JSONEncoder` / `JSONDecoder` implementations.

Part of the zero-dependency `stdlib` layer (only `encoding/json/v2` and
sibling stdlib packages).

## Top-level API

- `Marshal(v, opts...) ([]byte, error)`
- `MarshalIndent(v, prefix, indent string) ([]byte, error)`
- `MarshalWrite(w io.Writer, v, opts...) error`
- `Unmarshal(b []byte, v) error`
- `UnmarshalRead(r io.Reader, v) error`

If `v` implements `JSONEncoder` / `JSONDecoder`, those interfaces are
preferred; otherwise the standard `encoding/json/v2` path is used.

### Options

Options implement the sealed `MarshalOptions` interface. Built-in options:

- `Indent` / `IndentPrefix` — pretty-print indentation.
- `NilSliceAsNull` / `NilMapAsNull` — output nil collections as `null`
  (default `true`).
- `Deterministic` — sort map keys deterministically (default `true`).

## Streaming helpers

For values that implement `JSONEncoder` / `JSONDecoder`, `jsonflow` provides
per-scalar and structural helpers:

Encoders (`Encoder = json.Encoder`):

- `EncodeNull`, `EncodeBool[T]`, `EncodeInt[T]`, `EncodeUint[T]`,
  `EncodeFloat[T]`, `EncodeString[T]`, `EncodeBytes` (base64),
  `EncodeAny[T]`, `EncodeObject`.
- `EncodeArrayBegin` / `EncodeArrayEnd` / `EncodeArray` and
  `EncodeObjectBegin` / `EncodeObjectEnd` / `EncodeMap`.
- Map-key helpers: `EncodeIntKey`, `EncodeUintKey`, `EncodeStringKey`.
- Every scalar also has a `Ptr` variant that emits `null` for a `nil`
  pointer.

Decoders (`Decoder = json.Decoder`):

- `DecodeBool`, `DecodeInt[T]`, `DecodeUint[T]`, `DecodeFloat[T]`,
  `DecodeString`, `DecodeBytes` (base64), `DecodeAny[T]`, `DecodeObject`.
- `DecodeArray`, `DecodeMap` (higher-order combinators).
- `DecodeObjectBegin` / `DecodeObjectEnd` / `DecodeEOF` for framing.
- `Parse*` counterparts for use inside custom `parseFn` callbacks.
- Every scalar has a `Ptr` variant that returns `nil` on JSON `null`.

## Example

```go
import "go-spring.org/stdlib/jsonflow"

type User struct {
    Name string
    Age  int
}

func (u *User) EncodeJSON(e jsonflow.Encoder) error {
    if err := jsonflow.EncodeObjectBegin(e); err != nil { return err }
    if err := jsonflow.EncodeStringKey(e, "name"); err != nil { return err }
    if err := jsonflow.EncodeString(e, u.Name); err != nil { return err }
    if err := jsonflow.EncodeStringKey(e, "age"); err != nil { return err }
    if err := jsonflow.EncodeInt(e, u.Age); err != nil { return err }
    return jsonflow.EncodeObjectEnd(e)
}

b, _ := jsonflow.Marshal(&User{Name: "alice", Age: 30})
```
