# formutil
[English](README.md) | [中文](README_CN.md)

`formutil` provides generic encode / decode helpers between Go values and
form-style key-value maps (`url.Values`, `[]string`). Used by the Go-Spring
HTTP client / server binding code. Part of the zero-dependency `stdlib` layer.

## Features

- Symmetric `Decode<Type>` / `Encode<Type>` pairs for `bool`, signed and
  unsigned integers, floats, `string`, byte slices, and arbitrary JSON.
- `<Type>Ptr` variants that treat `nil` as "absent" on encode and return
  `*T` on decode.
- Generic `DecodeList` / `EncodeList` for repeated form fields.
- Overflow-safe integer / float decoding via `stdlib/mathutil`.
- JSON encoding delegates to `stdlib/jsonflow`.

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

## Rules

- Every non-list decoder rejects more than one raw value ("too many values
  for form field ...").
- Integer / unsigned / float decoders return a range error when the parsed
  value would not fit `T`.
- `DecodeBytes` / `EncodeBytes` use standard base64.
- `EncodeXxxPtr` omits the field entirely when the pointer is `nil`.
