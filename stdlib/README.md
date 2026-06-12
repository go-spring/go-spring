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

Each module is independent and can be used separately. Detailed documentation is available in each module's directory.

## Available Modules

| Module | Description |
|--------|-------------|
| [ctxcache](./ctxcache/) | Context-based caching utilities |
| [errutil](./errutil/) | Error handling utilities, provides error wrapping, stack trace capture and more |
| [fileutil](./fileutil/) | File system utilities |
| [flatten](./flatten/) | Flatten nested data structures |
| [formutil](./formutil/) | Form processing utilities |
| [funcutil](./funcutil/) | Function utilities, lazy evaluation, partial application and more |
| [goutil](./goutil/) | Generic Go language utilities, context cancellation control and more |
| [hashutil](./hashutil/) | Hashing utilities |
| [httpclt](./httpclt/) | HTTP client utilities |
| [httpsvr](./httpsvr/) | HTTP server utilities |
| [iterutil](./iterutil/) | Iterator and loop processing utilities |
| [jsonflow](./jsonflow/) | JSON streaming processing toolkit |
| [listutil](./listutil/) | List and linked list utilities |
| [mathutil](./mathutil/) | Math and numeric utilities |
| [md5util](./md5util/) | MD5 hashing convenience utilities |
| [netutil](./netutil/) | Network related utilities |
| [ordered](./ordered/) | Ordered map and set data structures |
| [patchutil](./patchutil/) | Patch processing utilities |
| [testing](./testing/) | Elegant fluent-style assertion library for unit testing, supports `assert` and `require` modes with type-specific assertions |
| [textstyle](./textstyle/) | Text style and formatting utilities |
| [typeutil](./typeutil/) | Type reflection and conversion utilities |

## License

Apache License 2.0
