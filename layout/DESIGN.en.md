# GS_PROJECT_NAME Design
[English](DESIGN.en.md) | [中文](DESIGN.zh.md)

This is the design note for the project scaffold `layout/` in the
Go-Spring monorepo. `layout/` is not a runnable app — it is the source
template that `gs init` clones (sparse-checkout), language-strips, feature-
prunes, and placeholder-substitutes into a new user project.

## 1. Responsibilities & Boundaries

- **Starting point, not a framework.** Everything under `layout/` is
  meant to be edited by the user after `gs init`. The template's job is
  to make the first commit sensible: correct module layout, real config
  keys, working `gs.Run()` wiring, and per-protocol IDL + server + init
  glue.
- **Domain-only layering.** `internal/` is fixed at `api/`,
  `application/`, `domain/`, `infra/`, `pkg/`, `consts/`, plus
  `init.go`. There is no `--layout` flag, no MVC / modulith variant
  directories, no form placeholder — domain layering is the only
  supported shape and this is deliberate.
- **Feature superset.** The template ships every framework/protocol
  server (`internal/api/server/*svr`), every IDL family (`idl/*`), every
  starter blank-import in `internal/init.go`. `gs init` prunes what the
  user didn't select. Downstream contributors extend the superset; they
  don't add "modes".

## 2. Key Conventions & Seams

- **Language variants.** Files ending `.en.md` / `.zh.md` are the source;
  `gs init --lang <lang>` strips the suffix so the user's project ends
  up with `AGENTS.md`, `README.md`, etc. Links inside the template that
  refer to `common-rules.md` / `domain-rules.md` **without** a language
  suffix are correct — they resolve after the strip. Do not "repair" them
  to add `.en` / `.zh`. The one deliberate exception is
  `coding-style.{en,zh}.md` mutual links, which are kept suffixed by user
  decision. This template README pair follows the same suffix rule
  (`README.en.md` / `README.zh.md`) and this DESIGN pair mirrors it
  (`DESIGN.en.md` / `DESIGN.zh.md`).
- **Per-protocol independent IDL.** Every RPC framework keeps its own
  IDL + code generator + native types. Controllers consume the framework's
  generated types (not the application's DTOs), and only the application
  service layer is shared across protocols. This is why the layout ships
  side-by-side `pb/`, `kitex_gen/`, `goctl` output — not a shared
  "protocol-neutral" DTO tree. Anything that looked like HTTP-shell for
  other protocols was rejected.
- **Framework-protocol naming & split.**
  - **`idl/` directories use dashes** between framework and protocol:
    `idl/goframe-grpc`, `idl/kitex-thrift`, `idl/kratos-ws`. `idl/` is
    not a Go package tree (real Go code lives under `pb/`, `kitex_gen/`
    subdirs), so dashes are allowed and improve readability.
  - **Server package directories use Go-idiomatic concat + `svr`
    suffix**: `goframegrpcsvr`, `kitexthriftsvr`, `kratoswssvr`.
    Go package names can't contain dashes.
  - **Single-protocol frameworks drop the suffix**: `trpc` has one
    protocol, so it is `idl/trpc` + `trpcsvr`, not `trpc-xxx`.
  - **Multi-protocol frameworks must split**: kitex is
    `kitex-thrift` + `kitex-grpc` with separate `spring.kitex.thrift.server`
    / `spring.kitex.grpc.server` config prefixes.
  - **Each server binds a distinct port**, hard-coded in
    `conf/app.properties`, even when the underlying starter is a stub.
    Ports are globally unique across the layout.
- **Placeholders.** `GS_PROJECT_MODULE`, `GS_PROJECT_NAME`,
  `GS_PROJECT_LANG`, `GS_LAYOUT_VERSION` — `gs init` substitutes these
  longest-key-first. Template files must reference them literally; do
  not pre-rewrite before pruning.

## 3. Constraints

- Layout files (Makefile, `docker-compose.yml`, `.yaml`, `.properties`,
  markdown) **do not** carry the Apache license header — this matches
  every non-`.go` file convention in the repo. Only generated `.go`
  files add the header.
- Do not add `--layout` / `-mvc` / `-modulith` variants. The
  "modulith" language in the framework README refers to Go-Spring's
  modular philosophy, not a layer variant of this template.
- Do not introduce cross-protocol shared IDL trees. Each protocol brings
  its own IDL, its own generator, its own native types.
- Ports allocated in `conf/app.properties` must not overlap — the layout
  is where the invariant is enforced, not the starters.

## 4. Trade-offs & Alternatives Rejected

- **Assembly-from-parts rejected.** A "build your own layout" wizard
  would explode the compatibility matrix between init-imports, feature
  flags, and config keys. The prune-a-superset strategy keeps every
  intermediate state a working project.
- **Multi-layering (MVC / modulith / domain choices) rejected.** Domain
  layering is the only supported shape (2026-07 user decision). Removing
  the choice removed placeholder variants and stripLayoutSuffix
  machinery.
- **HTTP-shell for non-HTTP protocols rejected.** Making every framework
  reuse the HTTP DTO tree loses the point of independent protocol
  ecosystems.
