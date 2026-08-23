# formutil

[English](README.md) | [中文](README_CN.md)

`formutil` provides generic encode / decode helpers between Go values and
form-style key-value maps (`url.Values`, `[]string`), used by the Go-Spring
HTTP client / server binding code.

## Usage

```go
import (
    "net/url"
    "go-spring.org/stdlib/formutil"
)

// Decode a single value
n, err := formutil.DecodeInt[int]("page", []string{"3"})

// Decode repeated values
ids, err := formutil.DecodeList("ids",
    []string{"1", "2", "3"}, formutil.DecodeInt[int64])

// Encode into url.Values
v := url.Values{}
_ = formutil.EncodeString(v, "name", "alice")
_ = formutil.EncodeIntPtr[int64](v, "opt", nil) // omitted
```

### API

#### Decode (`[]string` → Go values)

| Function | Description |
|---|---|
| `DecodeBool(key string, values []string) (bool, error)` | Decode a `bool` |
| `DecodeBoolPtr(key string, values []string) (*bool, error)` | Decode into `*bool` |
| `DecodeInt[T ~int64\|~int32\|~int16\|~int8\|~int](key, values) (T, error)` | Decode a signed integer, overflow-checked |
| `DecodeIntPtr[...](...) (*T, error)` | Decode into `*T` |
| `DecodeUint[T ~uint64\|~uint32\|~uint16\|~uint8\|~uint](...) (T, error)` | Decode an unsigned integer, overflow-checked |
| `DecodeUintPtr[...](...) (*T, error)` | Decode into `*T` |
| `DecodeFloat[T ~float64\|~float32](...) (T, error)` | Decode a float, overflow-checked |
| `DecodeFloatPtr[...](...) (*T, error)` | Decode into `*T` |
| `DecodeString(key, values) (string, error)` | Decode a `string` |
| `DecodeStringPtr(key, values) (*string, error)` | Decode into `*string` |
| `DecodeBytes(key, values) ([]byte, error)` | Decode a byte slice (standard base64) |
| `DecodeJSON[T any](key, values) (T, error)` | Decode arbitrary JSON (delegates to `stdlib/jsonflow`) |
| `DecodeList[T any](key string, values []string, fn Decoder[T]) ([]T, error)` | Decode repeated fields one by one via `fn` |

#### Encode (Go values → `url.Values`)

| Function | Description |
|---|---|
| `EncodeBool(m url.Values, key string, val bool) error` | Encode a `bool` |
| `EncodeBoolPtr(m, key, val *bool) error` | Omit the field when `nil` |
| `EncodeInt[T ~int64\|~int32\|~int16\|~int8\|~int](m, key, val T) error` | Encode a signed integer |
| `EncodeIntPtr[...](m, key, val *T) error` | Omit the field when `nil` |
| `EncodeUint[T ~uint64\|~uint32\|~uint16\|~uint8\|~uint](m, key, val T) error` | Encode an unsigned integer |
| `EncodeUintPtr[...](m, key, val *T) error` | Omit the field when `nil` |
| `EncodeFloat[T ~float64\|~float32](m, key, val T) error` | Encode a float |
| `EncodeFloatPtr[...](m, key, val *T) error` | Omit the field when `nil` |
| `EncodeString(m, key, val string) error` | Encode a `string` |
| `EncodeStringPtr(m, key, val *string) error` | Omit the field when `nil` |
| `EncodeBytes(m, key, val []byte) error` | Encode a byte slice (standard base64) |
| `EncodeJSON[T any](m, key, val T) error` | Encode arbitrary JSON (delegates to `stdlib/jsonflow`) |
| `EncodeList[T any](m, key string, values []T, fn Encoder[T]) error` | Encode a slice one by one via `fn` |

### Rules

- Every non-list decoder rejects more than one raw value ("too many values
  for form field ...") and rejects an empty value list ("missing value for
  form field ...").
- Integer / unsigned / float decoders return a range error when the parsed
  value would not fit `T`.
- `DecodeBytes` / `EncodeBytes` use standard base64.
- `EncodeXxxPtr` omits the field entirely when the pointer is `nil`.

## License

Apache License 2.0. See [LICENSE](../../LICENSE).
