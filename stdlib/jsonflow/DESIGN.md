# jsonflow Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. `jsonflow` is the JSON boundary
of Go-Spring: everything the framework marshals or unmarshals goes through
it, so it owns the defaults and the streaming seams.

## 1. Responsibilities & Boundaries

- Provide a single JSON entry point (`Marshal` / `Unmarshal` and their
  streaming variants) so the whole codebase picks the same defaults —
  deterministic key order, nil collections as `null`.
- Expose typed, per-token encoding / decoding helpers so hand-written or
  generated code can implement `JSONEncoder` / `JSONDecoder` without
  reflecting.
- Not a schema library. Field ordering, discovery, and validation are the
  caller's concern; `jsonflow` only understands raw tokens plus the two
  interfaces.

## 2. Key Abstractions & Seams

- **`JSONEncoder` / `JSONDecoder`**: opt-in hook for values that want to
  own their wire format. `Marshal` / `UnmarshalRead` type-assert first,
  falling back to `encoding/json/v2`. This is the primary seam used by
  code-generated types.
- **Sealed `MarshalOptions`**: an unexported `NotForPublicUse{}` argument on
  `JSONOptions` keeps the option set closed. New options ship as new
  package-level types (`Indent`, `NilSliceAsNull`, etc.). This trades user
  extensibility for API stability.
- **Deterministic defaults**: `NilSliceAsNull(true)`, `NilMapAsNull(true)`
  and `Deterministic(true)` are always applied first, before user options
  can override them. Chosen to make golden-file tests and cache keying
  stable across runs.
- **Generic scalar helpers**: `EncodeInt[T ~int|...]` avoids reflection at
  the leaf level. Combined with `mathutil.Overflow*`, decoders reject
  out-of-range numbers before they widen silently.
- **Higher-order combinators**: `DecodeArray[T](parseFn)` and
  `DecodeMap[K,V](parseKey, parseVal)` let generated code compose per-type
  decoders without capturing framework state.

## 3. Constraints & Trade-offs

- Depends on `encoding/json/v2` — Go 1.26+ only. The v1 fallback lives in
  `internal/json`, which the streaming helpers program against.
- `EncodeFloat` maps `NaN`, `+Inf`, `-Inf` to the strings `"NaN"`,
  `"Infinity"`, `"-Infinity"` respectively. This is intentional to keep
  output valid JSON, but it means round-tripping requires the caller's
  decoder to know that convention.
- `DecodeBytes` treats `null` as "return nil, no error", while
  `DecodeString` treats `null` as an error. Bytes are commonly optional;
  strings usually are not, and the shape reflects that.
- Numeric decoders accept map keys as both `"..."` and `0` tokens through
  the `ParseIntKey` / `ParseUintKey` variants — necessary because
  `encoding/json/v2` renders numeric map keys as strings.
