# md5util Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. Kept separate from `hashutil`
so that using MD5 is an explicit, visible import.

## 1. Responsibilities & Boundaries

- Give the caller `MD5(string) string` in one call, in the hex form nearly
  every consumer needs (cache keys, ETags, fingerprints).
- Not a HMAC or streaming API. Callers doing chunked hashing or key-derivation
  should use `crypto/md5` (or, preferably, a modern hash) directly.

## 2. Design Notes

- Output is lowercase hex via `encoding/hex.EncodeToString`. This form
  matches most database / cache conventions.
- Making the package one function is the point: any expansion (SHA-1, SHA-256,
  HMAC) belongs in a differently named package so callers "opt in" by import.
