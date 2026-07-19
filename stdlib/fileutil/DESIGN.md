# fileutil Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. Two tiny helpers that make `os`
call sites less verbose.

## 1. Responsibilities & Boundaries

- Collapse the "check for `os.ErrNotExist`" pattern into a single call
  (`PathExists`), so the boolean and the error stay clearly separated.
- Read directory entry names without leaking an `*os.File` — `ReadDirNames`
  opens, reads, and closes internally.
- Not a filesystem abstraction. No walking, watching, atomic write, or path
  manipulation helpers live here; those are either in `os`/`filepath` or in
  higher-layer packages.

## 2. Design Notes

- `PathExists` never returns `os.ErrNotExist`; not-exists is expressed as
  `(false, nil)`. Any other stat error is bubbled up untouched.
- `ReadDirNames` returns whatever `f.Readdirnames(-1)` produces (may return a
  partial slice together with a non-nil error). Callers must check both.
