# gs-gui Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`gs-gui` is an external `gs` tool in the tooling layer of the four-layer
stack (stdlib → spring → starter → gs). It exposes a browser-based
wizard that is a thin front-end over `gs init`.

## 1. Responsibilities & Boundaries

- Serve a single-page HTML wizard, collect `module` + `lang`, then exec
  the sibling `gs` binary with `init -m <module> --lang <lang>` and
  stream its combined stdout/stderr back to the browser.
- Does not implement project generation. Every generation decision lives
  in `gs init`; `gs-gui` is purely presentational so behavior stays
  consistent between CLI and GUI users.

## 2. Key Abstractions & Seams

- **External-tool protocol.** The binary is named `gs-gui`, lives beside
  the `gs` binary, and prints a two-line description/version for
  `gs-gui --version`. `gs gui` dispatches to it via the same lookup as
  every other external tool (see `gs/gs/tool/tool.go`).
- **Embedded UI.** `//go:embed web/index.html` — the wizard is a single
  file compiled into the binary, so there is no separate asset step.
- **Sibling-binary discovery.** `siblingGS()` resolves
  `filepath.Dir(os.Executable()) + "/gs"`; `gs-gui` refuses to run if
  that binary is missing, keeping the "GUI is a shell over CLI"
  invariant.
- **Port selection.** `defaultPort=8639`; on `EADDRINUSE`
  `net.Listen("tcp", "127.0.0.1:0")` picks an ephemeral port. Bind is
  always `127.0.0.1` — this is a local dev tool, not a service.
- **Streaming response.** `POST /api/create` merges stderr into stdout,
  reads 1 KB at a time from the pipe, and `Flush()`es after each write.
  The response is `text/plain` with `X-Content-Type-Options: nosniff` so
  the browser renders it as an append-only log.

## 3. Constraints

- Never re-implement `gs init` logic here. Any new option (feature flags,
  language variants, layout tag pinning) must land in the CLI first so
  the two entry points cannot drift.
- Listener is always `127.0.0.1`. Do not expose it on `0.0.0.0` — the
  tool exec's a project scaffolder that writes to disk in the invoker's
  cwd.

## 4. Trade-offs & Alternatives Rejected

- **Websockets rejected.** Chunked `text/plain` + `Flusher` is simpler
  and browser-native; the wizard only needs one-way progress streaming.
- **No API surface beyond `/api/create`.** Feature enumeration, add-flow,
  etc. are deliberately absent — this keeps the GUI a demo/onboarding
  aid rather than a competing UX with its own state.
