# httpsvr Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`httpsvr` supplies the server-side shape that generated handlers plug into. It
stays in `stdlib`'s zero-dependency layer: everything is `net/http` plus the
in-repo `jsonflow` / `ctxcache` / `errutil` utilities.

## 1. Responsibilities and Boundaries

- Provide a routing seam (`Server.Route`) that a starter can implement over any
  underlying router; ship a `SimpleServer` backed by `http.ServeMux`.
- Wrap generic JSON and SSE handlers so a generated Go 1.22+ handler is a small
  adapter, not a re-implementation of parsing, validation and framing.
- Refuse to become a full web framework. There is no middleware chain, no
  binding tag magic, no dependency injection — those belong at the starter
  layer or in user code.

## 2. Key Abstractions and Seams

- **`Server` interface** — one method, `Route(Router)`. A starter that wants a
  different router (chi, gin, ...) implements this and everything else keeps
  working.
- **`RequestContext`** — the request/response pair, stored via
  `WithRequestContext` so a handler that received `context.Context` (through
  `ctxcache` or friends) can still reach the writer.
- **`ReadBody` and `ErrorHandler` variables** — deliberately mutable so an
  application can enforce a smaller body cap or a JSON error envelope without
  wrapping `HandleJSON`.
- **`RequestObject` interface (`Bind` + `Validate`)** — the contract with
  generated request types. `ReadRequest` picks JSON vs form by `Content-Type`,
  falling back to a first-byte sniff so an unlabelled body still parses.

## 3. Constraints

- Bodies are read only for `POST` / `PUT` / `PATCH`. Any other method skips
  `decodeBody`; a `GET`-with-body request is treated as bodyless.
- `HandleStream` requires the `http.ResponseWriter` to implement
  `http.Flusher`; it reports a 500 through `ErrorHandler` otherwise. Wrapping
  writers must not hide flushing.
- The response `Content-Type` for JSON is set to `application/json` before the
  handler runs, so a business handler cannot forget it.
- `RequestObject.Bind` runs after body decode; a body-decode error short-circuits
  before `Bind`, so `Bind` may assume decoded fields are already populated for
  body-carrying methods.

## 4. Trade-offs and Alternatives Rejected

- **No custom router.** Go 1.22 gave `http.ServeMux` method-aware patterns,
  which is enough for the seam-level responsibility of this package; adding a
  third-party router would violate the zero-dependency rule.
- **No middleware slice.** Chains belong at higher layers (`aspect`, the
  `security` middleware, a starter's own `MiddlewareContributor`). Baking one
  in here would force users into that ordering.
- **Two encoding paths (JSON / form) with a first-byte sniff.** More flexible
  content negotiation is deferred: real APIs either use JSON or `x-www-form-
  urlencoded`, and the sniff covers the common case of a missing header.
