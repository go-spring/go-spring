# stdlib

<div>
   <img src="https://img.shields.io/github/license/go-spring/stdlib" alt="license"/>
   <img src="https://img.shields.io/github/go-mod/go-version/go-spring/stdlib" alt="go-version"/>
   <img src="https://img.shields.io/github/v/release/go-spring/stdlib?include_prereleases" alt="release"/>
   <a href="https://codecov.io/gh/go-spring/stdlib" >
      <img src="https://codecov.io/gh/go-spring/stdlib/branch/main/graph/badge.svg?token=SX7CV1T0O8" alt="test-coverage"/>
   </a>
   <a href="https://goreportcard.com/report/go-spring.org/stdlib">
      <img src="https://goreportcard.com/badge/go-spring.org/stdlib" alt="Go Report Card"/>
   </a>
   <a href="https://deepwiki.com/go-spring/stdlib"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</div>

[English](README.md) | [中文](README_CN.md)

`stdlib` is a collection of high-quality independent utility modules written in Go.
It provides carefully crafted tools that complement the Go standard library,
making everyday Go development more convenient and enjoyable.

## Available Modules

### Web & Network

| Module | Description |
|--------|-------------|
| [httpclt](./httpclt/) | Runtime toolkit for declarative HTTP clients: metadata, request options and streaming JSON helpers |
| [httpsvr](./httpsvr/) | Thin HTTP server toolkit: `ServeMux`-based server seam, request context, JSON and SSE handler wrappers |
| [httputil](./httputil/) | OTel-free HTTP semantic-convention attributes derived from an inbound request, shared by server starters |
| [formutil](./formutil/) | Form processing utilities: decode form values into typed structs |
| [netutil](./netutil/) | Network related utilities |

### JSON & Data Shaping

| Module | Description |
|--------|-------------|
| [jsonflow](./jsonflow/) | JSON streaming processing toolkit |
| [flatten](./flatten/) | Flatten nested data structures into flat key paths |
| [patchutil](./patchutil/) | Patch processing utilities |

### Collections & Generics

| Module | Description |
|--------|-------------|
| [goutil](./goutil/) | Generic Go utilities, context cancellation control and more |
| [iterutil](./iterutil/) | Iterator and loop processing utilities |
| [listutil](./listutil/) | Generic, type-safe skin over `container/list` plus slice helpers |
| [ordered](./ordered/) | Sorted map-key iteration helper |

### Errors, Context & Files

| Module | Description |
|--------|-------------|
| [errutil](./errutil/) | Error handling utilities: error wrapping, stack trace capture, constructor preconditions |
| [ctxcache](./ctxcache/) | Context-based request-scoped caching utilities |
| [fileutil](./fileutil/) | File system utilities |
| [funcutil](./funcutil/) | Function utilities: lazy evaluation, partial application and more |

### Text, Hash & Math

| Module | Description |
|--------|-------------|
| [textstyle](./textstyle/) | Text style and formatting utilities |
| [hashutil](./hashutil/) | Hashing utilities |
| [md5util](./md5util/) | MD5 hashing convenience utilities |
| [mathutil](./mathutil/) | Math and numeric utilities, including overflow-safe arithmetic |
| [bufutil](./bufutil/) | Bounded buffers for side-channel copying that never backpressure the primary flow |
| [typeutil](./typeutil/) | Type reflection and conversion utilities |

### Testing

| Module | Description |
|--------|-------------|
| [testing](./testing/) | Fluent assertion library with `assert` (fail-continue) and `require` (fail-fast) modes and type-specific assertions |

## License

Apache License 2.0
