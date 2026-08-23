# jsonflow

[English](README.md) | [中文](README_CN.md)

`jsonflow` is Go-Spring's streaming JSON layer, built on Go 1.26's
`encoding/json/v2` + `encoding/json/jsontext`. It is the framework's single
JSON boundary: everything marshaled or unmarshaled goes through it, so the
whole codebase shares the same defaults (deterministic map key order, nil
map / slice as `null`). On top of a drop-in `Marshal` / `Unmarshal` /
`MarshalWrite` / `UnmarshalRead` API it offers generic `Encode<T>` /
`Decode<T>` helpers for hand-written streaming encoders / decoders, generated
code, and custom `JSONEncoder` / `JSONDecoder` implementations.

## Usage

Import path:

```go
import "go-spring.org/stdlib/jsonflow"
```

### API

- `Marshal(v, opts...) ([]byte, error)`
- `MarshalIndent(v, prefix, indent string) ([]byte, error)`
- `MarshalWrite(w io.Writer, v, opts...) error`
- `Unmarshal(b []byte, v) error`
- `UnmarshalRead(r io.Reader, v) error`
- `NewEncoder(w io.Writer) Encoder` / `NewDecoder(r io.Reader) Decoder` —
  streaming encoder / decoder for hand-written `JSONEncoder` / `JSONDecoder`
  implementations.

If `v` implements `JSONEncoder` / `JSONDecoder`, those interfaces are
preferred — an opt-in hook for values that want to own their wire format,
used primarily by code-generated types; otherwise the standard
`encoding/json/v2` path is used.

### Options

Options implement the sealed `MarshalOptions` interface. Built-in options:

- `Indent` / `IndentPrefix` — pretty-print indentation.
- `NilSliceAsNull` / `NilMapAsNull` — output nil collections as `null`
  (default `true`).
- `Deterministic` — sort map keys deterministically (default `true`).

### Streaming helpers

For values that implement `JSONEncoder` / `JSONDecoder`, `jsonflow` provides
per-scalar and structural helpers:

Encoders (`Encoder = json.Encoder`):

- `EncodeNull`, `EncodeBool[T]`, `EncodeInt[T]`, `EncodeUint[T]`,
  `EncodeFloat[T]`, `EncodeString[T]`, `EncodeBytes` (base64),
  `EncodeAny[T]`, `EncodeObject`.
- `EncodeArrayBegin` / `EncodeArrayEnd` / `EncodeArray` and
  `EncodeObjectBegin` / `EncodeObjectEnd` / `EncodeMap`.
- Map-key helpers: `EncodeIntKey`, `EncodeUintKey`, `EncodeStringKey`.
- Every scalar except Bytes also has a `Ptr` variant that emits `null` for a
  `nil` pointer.

Decoders (`Decoder = json.Decoder`):

- `DecodeBool`, `DecodeInt[T]`, `DecodeUint[T]`, `DecodeFloat[T]`,
  `DecodeString`, `DecodeBytes` (base64), `DecodeAny[T]`, `DecodeObject`.
- `DecodeArray`, `DecodeMap` are higher-order combinators:
  `DecodeArray[T](parseFn)` and `DecodeMap[K,V](parseKey, parseVal)` compose
  per-type decoders without capturing framework state.
- `DecodeObjectBegin` / `DecodeObjectEnd` / `DecodeEOF` for framing.
- Map-key decoders: `DecodeIntKey`, `DecodeUintKey`.
- `Parse*` counterparts for use inside custom `parseFn` callbacks.
- Every scalar except Bytes has a `Ptr` variant that returns `nil` on JSON
  `null`; `DecodeBytes` handles `null` itself.

The scalar helpers are generic: `EncodeInt[T ~int|...]` and friends avoid
reflection at the leaf level; combined with `mathutil.Overflow*`, decoders
reject out-of-range numbers before they widen silently.

A few edge shapes are worth noting:

- `EncodeFloat` maps `NaN`, `+Inf`, `-Inf` to the strings `"NaN"`,
  `"Infinity"`, `"-Infinity"` respectively. This keeps output valid JSON,
  but round-tripping requires the caller's decoder to know that convention.
- `DecodeBytes` treats `null` as "return nil, no error", while
  `DecodeString` treats `null` as an error. Bytes are commonly optional;
  strings usually are not, and the shape reflects that.
- Numeric decoders accept map keys as both `"..."` and `0` tokens through
  the `ParseIntKey` / `ParseUintKey` variants — necessary because
  `encoding/json/v2` renders numeric map keys as strings.

### Example

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

## Dependencies & compatibility

Depends on `encoding/json/v2` — Go 1.26+ only, no v1 fallback. The streaming
helpers program against `internal/json`, a vendor-neutral seam of token
interfaces (Encoder, Decoder, Kind); `internal/jsonv2` is its sole adapter,
implemented on top of `encoding/json/v2`.

## License

Apache License 2.0. See [LICENSE](../../LICENSE).
