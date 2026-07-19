# formutil Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. `formutil` is the generic bridge
between HTTP form / query strings and Go values used by higher-layer binders.

## 1. Responsibilities & Boundaries

- Provide **primitive**, single-field encode / decode helpers. Struct-level
  binding lives in the caller (HTTP framework binder / declarative client).
- Ship symmetric encode / decode pairs so that the same code that emits a
  form field can also parse it back.
- Not a validator. Range check is the only cross-cutting rule that lives
  here (via `mathutil.Overflow*`), everything else is delegated.

## 2. Key Design Decisions

- **Generic function set** (`Decode/EncodeInt[T]`) instead of type-per-file.
  This keeps the surface flat and lets a code-generator target one function
  per field type.
- **Same `[]string` shape for input on decode**, matching `url.Values[key]`.
  Decoders complain when the slice has more than one element unless they
  are the `List` variants.
- **`nil` pointer as absent** on the encode side. The `Ptr` variants let a
  binder distinguish "unset" from "zero" without a parallel bitmap.
- **Base64 for bytes**, **JSON via `stdlib/jsonflow`**. Both choices lock in
  a canonical wire format so the endpoints of a binder pair agree by
  construction.

## 3. Constraints

- Zero non-`stdlib` dependencies.
- `EncodeFloat` / `DecodeFloat` use `strconv.FormatFloat(..., 'f', -1, 64)`
  regardless of the underlying type width; the caller loses precision info
  if `T = float32`.
- Overflow errors are returned as plain messages via `errutil.Explain`; they
  are not typed sentinels — callers should test with string matching or wrap
  further up.
