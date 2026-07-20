# session Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`session` is the zero-dependency stdlib abstraction for server-side HTTP
sessions. Distributed backends live in starters (`starter-session-redis`);
the bundled `Memory` store keeps stdlib self-testing.

## 1. Responsibilities & Boundaries

- Own the session lifecycle from the moment a request enters the manager
  middleware to the moment its response headers are written: load / create /
  attach / mutate / write-back / rotate / destroy.
- Not the identity provider. Session attributes are arbitrary; who a caller
  is comes from `spring/security`.
- Not a distributed store. `SessionStore` is the seam; remote stores
  contribute a bean implemented on `ByteStore`.

## 2. Key Abstractions & Seams

- `Session` — state + non-persistent flags (`isNew`, `modified`, `invalid`,
  `renew`) consumed by the Manager at write-back time. `snapshot()` copies
  the persistable portion for the store to serialize.
- `SessionStore` — `Load` / `Save(ttl)` / `Delete`. The **`ByteStore` +
  `FromByteStore`** seam is the equivalent of `cache.ByteStore`: a starter
  implements the narrow `Get/Set/Delete []byte` interface over its client
  and gets a full `SessionStore` via JSON encoding of `sessionData`. Every
  remote backend shares one serialization path.
- Package-level `Register` / `Get` / `MustGet` — driver-registry idiom for
  process-static stores. `Memory` is registered as `"memory"` in `init()`.
- `Manager` — the single HTTP seam. `Middleware` reads the cookie, loads or
  creates the session, attaches it via `WithSession(ctx)`, and installs a
  `sessionWriter` that commits **before** the first `WriteHeader`/`Write`.

## 3. Constraints (do not break)

- **`Set-Cookie` must precede body**. `sessionWriter.commit()` runs from
  both `WriteHeader` and `Write`, and once at middleware exit for handlers
  that never wrote. It runs at most once (`committed` guard). Attribute
  changes after the first write are not persisted — the same constraint any
  header carries.
- **Lazy id + lazy store entry**. A brand-new, untouched session is left
  alone (`!modified && !hadID` → no id, no `Save`, no cookie). Anonymous
  traffic must not allocate a session.
- **Sliding renewal**. Every request that carries a session must `Save` to
  refresh the store TTL and the cookie `Max-Age`, even if attributes did not
  change (`hadID` alone triggers persistence).
- **RenewID = delete + regen**. On `renew && hadID` the old id is deleted
  from the store before a fresh id is generated; attributes are preserved.
  This is the anti-fixation guarantee.
- **Cookie is always `HttpOnly`**. Not configurable — a JS-readable session
  cookie is almost always a bug and a bool zero cannot distinguish an
  explicit `false` from "unset".
- **id entropy**: 32 bytes from `crypto/rand`, base64url-encoded.
- **Remote backends do not go through the registry**. A live Redis client
  cannot live in a package-global map across tests and restarts; contribute
  a `SessionStore` bean instead. The registry is for process-static defaults
  (like `Memory`).

## 4. Trade-offs / Alternatives Rejected

- **JSON, not gob**, for `ByteStore` serialization: cross-language readable,
  no version surprises, keeps attributes JSON-friendly by construction.
- **No `Session` mutation after first write silent-drop**. It is *not*
  raised as an error — the same silent-drop as any header written late. We
  document it; callers change attributes early.
- **`Save` mid-response failure suppresses cookie for brand-new session**.
  We would rather hand no cookie than one whose store entry is missing;
  the request otherwise completes normally.
- **Distributed backend as a bean, not a registry entry**. Mirrors
  `starter-lock-redis` decision; a live client belongs to the container,
  not a package-global map.
