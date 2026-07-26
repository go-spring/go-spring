# starter-swagger Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-swagger` is a **Contributor**-archetype starter (see
[starter/DESIGN.md](../DESIGN.md) §2.3) that serves a Swagger UI over an
already-generated `openapi.json`. It opens no port; when
`starter-actuator` is present it auto-mounts on the actuator; otherwise
the application mounts the returned `http.Handler`.

## 1. Responsibilities & Boundaries

- **In scope:** load the OpenAPI 3.0 spec at startup, render a small
  HTML shell that pulls Swagger UI assets from a CDN, and expose the
  bundle as an `endpoint.Endpoint` for actuator auto-mount.
- **Out of scope:** OpenAPI *generation* (that is `gs-http-gen --openapi`
  which produces `openapi.json`); embedded static assets; the older
  Swagger 2.0 `--swagger` spec (intentionally unsupported).

## 2. Key Decisions

- **`endpoint.Endpoint` shape for zero-wiring actuator mount.** `Path()`
  returns a trailing-slash base path; actuator uses `mux.Handle` on
  the subtree, so the entire `/swagger/*` surface attaches
  automatically. The same struct is a plain `http.Handler` too, so an
  application without actuator can `gs.Provide` it and mount it
  wherever.
- **CDN assets, not bundled statics.** `assetBaseURL` (default
  `unpkg swagger-ui-dist@5`) is loaded at runtime. Bundling ~1MB of
  minified UI in every binary is a poor default; users on air-gapped
  networks override the CDN URL.
- **Fail-fast on spec read.** The spec file is read once at startup;
  a missing or malformed file is a boot error, not a stale-UI symptom
  on first request.

## 3. Constraints

- **`html/template` JS escaping.** In JS context the template escapes
  `/` as `\/` (still valid JS). Smoke tests should assert `openapi.json`
  as a substring, not the literal `/swagger/openapi.json` path — the
  escape hit tests during development.
- **Swagger 2.0 (`--swagger`) unsupported on purpose.** OpenAPI 3.0 is
  the modern shape; keeping the older spec would double the surface
  for little value. `gs-http-gen`'s `--swagger` flag remains available
  for users who need it, but this UI does not consume it.

## 4. Trade-offs / Alternatives Rejected

- **Embed Swagger UI statics via `embed.FS` — rejected.** Bloats every
  binary and complicates upgrades. CDN pinning by version tag
  (`@5`, `@5.x.x`) gives reproducibility without shipping bytes.
- **A separate `gs.Server` on its own port — rejected.** Docs traffic
  is low-volume and dev-facing; actuator (or an existing app mux) is
  the right home. Its sibling `starter-admin-ui`, on the other hand,
  uses `gs.Server` on `:9280` because it is doing *poller* work and
  should not race the application's business port.
