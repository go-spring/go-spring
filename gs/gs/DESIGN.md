# gs Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`gs` is the Toolkit Manager binary in the tooling layer of the Go-Spring
four-layer stack (stdlib → spring → starter → gs). It has three purposes:
scaffold new Go-Spring projects from the `layout/` superset, generate code
from a project's `idl/`, and produce Kubernetes deploy scaffolding. Any
subcommand not compiled in is dispatched to a sibling `gs-<tool>` binary
(external-tool protocol).

## 1. Responsibilities & Boundaries

- **Built-in subcommands** (`gs/gs/main.go` `builtins` map):
  - `gs init` — clone the latest `layout/vX.Y.Z` tag with sparse-checkout,
    pick a doc language, prune unselected features, substitute
    `GS_PROJECT_*` placeholders, then run `gs gen`.
  - `gs gen` — walk `idl/` under a project root (marked by `gs.json`) and
    dispatch each protocol subdirectory to its generator (currently
    `idl/http` → `gs-http-gen` via `cmd/proto`).
  - `gs add` — copy an additional feature slice from the layout tag pinned
    in `gs.json` into an existing project.
  - `gs k8s` — render Kubernetes deploy scaffolding into the current
    project.
  - `gs go` / `gs serve` — dev-loop wrappers around `go build`/`go run`.
- **External tools** — any `gs <name>` not in `builtins` execs `gs-<name>`
  next to the `gs` binary (see `gs/gs/tool/tool.go`). `gs-mock`,
  `gs-http-gen`, `gs-gui` follow this protocol.
- The CLI does **not** run at application runtime. Every file it produces
  (Dockerfile, manifests, `conf/app-k8s.properties`, layout files) is an
  editable starting point, never a runtime dependency of the generated
  project.

## 2. Key Abstractions & Seams

- **Embedded feature manifest.** `gs init`'s feature flags come from
  `cmd/feature/features.json` compiled in via `//go:embed features.json`
  (`cmd/feature/embed.go`). This is the source of truth: cobra registers
  flags before argv is parsed, so the manifest must be baked into the
  binary; adding a feature = edit JSON + rebuild `gs`. The manifest must
  stay in sync with the layout superset it prunes.
- **Layout-as-superset + prune model.** `layout/` on the remote is the
  full "family pack": every IDL, every server, every controller variant,
  every starter blank-import. `gs init` clones it (sparse-checkout,
  `--filter=blob:none`, `--depth 1`, `--branch layout/vX.Y.Z`), strips
  language suffix (`.en`/`.zh`) per `--lang`, and `feature.Prune` deletes
  everything the user didn't select. Placeholder substitution
  (`GS_PROJECT_MODULE`, `GS_PROJECT_NAME`, `GS_PROJECT_LANG`,
  `GS_LAYOUT_VERSION`) runs longest-key-first so shorter keys never
  clobber longer ones.
- **Feature flag rules** (see `cmd/feature/feature.go`, memory
  `project_gs_init_feature_flags.md`). Bare flag = default slice; the
  same flag key names the same vertical slice (idl + server + controller
  variant + converter + `init.go` import + starter). Only multi-protocol
  frameworks carry a protocol suffix (`--kitex-thrift`, `--kitex-pb`).
  Runtime configuration (addr, db, pool sizes) never becomes a flag —
  those live in generated `conf/`. Features are declared strictly
  independent; there is no cross-feature dependency system.
- **Embedded K8s templates.** `cmd/k8s/embed.go` uses
  `//go:embed all:templates` — the `all:` prefix is required so dotfiles
  like `.dockerignore` are embedded (default `embed` skips names starting
  with `.` or `_`). `k8s.Write` renders every template into the project,
  applies placeholder replacements longest-key-first, and skips existing
  files unless `--force`. Substitutions include `GS_APP_NAME` (derived
  via `toDNS1123(moduleLeaf)`), `GS_APP_PORT` (`--port`, default 9090),
  `GS_MGMT_PORT` (hardcoded 9370), `GS_IMAGE`.
- **Offline generation.** Neither the feature manifest nor the k8s
  templates are fetched at runtime; `gs init` still needs network (git
  clone) but `gs k8s` and `gs add` operate purely from embedded state /
  local project files.
- **Verbosity contract** (see `gs/CLAUDE.md`). Every entry point wires
  the shared `-v` count flag through `internal/runcmd.BindFlag`. Level 0
  prints `[INFO]` step lines; `-v` also logs argv; `-vv` streams child
  stdout/stderr live. Not optional — this is the debugging seam and
  cannot be simplified away.

## 3. Constraints

- The embedded `features.json` must stay in lockstep with the remote
  `layout/` superset. Drift means users get flags that reference paths
  the layout no longer ships.
- Placeholder replacement runs on raw layout bytes before `gs gen`, so
  `Owns` paths in the manifest and init-import lines both reference the
  token `GS_PROJECT_MODULE` — do not pre-rewrite those before pruning.
- Generated `deploy/` templates and Dockerfile carry no Apache license
  header (aligning with `layout/`'s Makefile / docker-compose); only
  generated `.go` files get the header.
- K8s probe wiring (`/startup`, `/health`, `/readiness` on port 9370,
  `preStop sleep 5`, `terminationGracePeriodSeconds 30`) is deliberately
  aligned with `stdlib/actuator` and framework graceful-drain defaults;
  editing template values in isolation will silently misalign shutdown.

## 4. Trade-offs & Alternatives Rejected

- **Assembly-from-parts rejected in favor of prune-from-superset.**
  Composing a project from independent chunks was considered and rejected
  because feature interactions (init.go imports, blank imports, config
  keys) explode combinatorially. A pruned superset stays a working
  project at every intermediate cut.
- **Runtime manifest fetch rejected.** Cobra needs flags at
  `RegisterFlags` time, which happens before argv is parsed and long
  before any network call. Embedding the manifest is the only reliable
  design.
- **Auto-composition of features rejected.** Features are independent by
  contract; no dependency edges. This keeps `--list-features` a flat menu
  and matches the "cut from full pack" mental model.
