# hashutil Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. Currently a one-function file
around `hash/fnv`.

## 1. Responsibilities & Boundaries

- Offer a single-call form of "hash a string to a uint64" so common
  bucketing / sharding sites do not repeat the `New64a` + `Write` + `Sum64`
  triplet.
- Not a cryptographic hash package. MD5 lives in its own `md5util`; SHA
  family and HMAC belong outside stdlib if they ever land.

## 2. Design Notes

- Delegates to `hash/fnv` rather than reimplementing the FNV-1a loop inline.
  Readability and consistency with any other `hash.Hash` user win over
  shaving a few nanoseconds.
- No streaming API. If a caller needs to feed many chunks incrementally they
  should use `hash/fnv` directly.
