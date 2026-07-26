# starter-session-redis Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-session-redis` is a **Contributor**-archetype starter (see
[starter/DESIGN.md](../DESIGN.md) §2.3) that provides a Redis-backed
`session.SessionStore` for `spring/session`, the Spring Session equivalent
in Go-Spring. It opens no port; it contributes a `SessionStore` bean that
`spring/session.Manager` uses to load and save sessions across replicas.

## 1. Responsibilities & Boundaries

- **In scope:** turn a `spring.session.redis.<name>` entry into a named
  `session.SessionStore` bean backed by the application's existing
  `*redis.Client`.
- **Out of scope:** the HTTP `Manager` middleware and cookie handling
  (that is `spring/session`); the redis client itself (that is
  `starter-go-redis`).

## 2. Key Abstractions & Seams

`spring/session` splits the capability three ways so the starter can plug
in without touching HTTP:

- **`Session`** — id, attributes, `isNew` / `modified` / `invalid` /
  `renew` state bits used by the middleware at write-back time.
- **`SessionStore`** — `Load` / `Save(ttl)` / `Delete`. Remote backends
  implement the narrower **`ByteStore`** and get lifted with
  `FromByteStore` (JSON-encoded `sessionData{Attributes, CreatedAt}`) —
  the same seam shape the cache stack uses.
- **`Manager`** — the sole HTTP seam. Middleware parses the cookie,
  loads the session, injects it into `context`, and writes back before
  the first header.

This starter contributes an implementation of `ByteStore` and lets
`FromByteStore` handle encoding, matching `starter-lock-redis` in shape.

## 3. Key Decisions

- **Bean, not driver registry.** Sessions rely on a *live* Redis client
  the application already wires; registering a live client into a
  package-level registry is wrong for multi-run and multi-test setups.
  The driver registry in `spring/session` only serves static defaults
  (`"memory"`); Redis rides through `gs.Group` + `TagArg(client)`.
- **Multi-instance via `gs.Group("${spring.session.redis}", ...)`.**
  Each entry picks a redis bean by name. Empty `client` = fail-fast at
  construction; no silent fallback to `localhost` (family rule §2.2).
- **`Store` embeds `session.SessionStore`.** `gs.Provide` cannot return
  an unexported implementation; embedding the interface exposes the
  concrete type while inheriting all methods from `FromByteStore`.
- **TTL is the Redis key TTL.** Redis enforces expiry and slide-renewal
  cheaply; no separate reaper is needed. `redis.Nil` maps to a miss.
- **No destroy hook.** The starter does not `Close()` the redis client
  (it does not own it); the redis starter's destroy takes care of that.

## 4. Manager decisions (context — lives in `spring/session`, mirrored here)

- **Cookie is always HttpOnly.** No configurable toggle: a JS-readable
  session cookie is almost always a bug, and a `bool` zero value cannot
  distinguish an explicit `false`.
- **Lazy allocation.** A never-modified new session does not persist and
  does not send a cookie; the id is generated only on first `Set`.
- **Sliding renewal.** Any request with a session refreshes both the
  Redis key TTL and the cookie `Max-Age`; idle beyond `IdleTimeout`
  expires it.
- **Fixation defense.** `Session.RenewID()` sets a bit; on commit the
  old id is deleted and a fresh one issued with attributes preserved.
- **Cookie precedes body.** `sessionWriter` wraps `ResponseWriter`;
  `commit()` runs once before the first `WriteHeader` / `Write` so
  `Set-Cookie` always precedes the body.

## 5. Trade-offs / Alternatives Rejected

- **Global driver registry with a live client — rejected.** Test / run
  isolation breaks when a live connection lives in a package-level map.
- **Per-implementation config prefix — rejected.** Following the family
  shared-prefix rule (`spring.session`), swapping between the in-memory
  default and this Redis backend is a blank-import change; nothing else
  moves.
