# randutil

[English](README.md) | [中文](README_CN.md)

`randutil` generates random identifier strings — the fencing tokens, request
ids, session ids, CSRF tokens and opaque OAuth2 values that otherwise litter a
codebase with hand-rolled `crypto/rand` snippets.

## Hex and URLSafe

- `Hex(n)` — 2n hex characters from n random bytes; `n=16` is the common
  short-id size (128 bits).
- `URLSafe(n)` — unpadded base64url of n random bytes; `n=32` (256 bits) is
  the common unguessable-token size.

Since Go 1.24 `crypto/rand` never fails, so neither do these.
