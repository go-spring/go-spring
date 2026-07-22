# Dependency Version Governance (BOM)

Go-Spring is a workspace of 60+ independent `go.mod` modules. Without a single
place to align versions, shared dependencies drift apart - and version skew has
already caused real breakage (a go1.26 toolchain suffix crashed codegen tools).

`versions.yaml` at the repo root plays the role Spring's BOM plays: it records
the third-party versions Go-Spring has "blessed", and the **maintainer-only**
`bomtool` (invoked through `scripts/versions.sh`) reports - and optionally
aligns - modules that deviate from that baseline.

> This is governance for the go-spring mono-repo itself. It is **not** a command
> in the `gs` toolkit users install, and is deliberately kept out of
> `gs --help` - a single-module user project has no `versions.yaml` and no
> `go.work`, so a workspace-BOM tool would only confuse them. Adopters running a
> similar `go.work` mono-repo may adapt it.

## versions.yaml

```yaml
go: "1.26"
disabled:
  go:
    - "1.26.0"        # runtime.Version() -X:jsonv2 suffix breaks codegen
dependencies:
  go.opentelemetry.io/otel: v1.43.0
  google.golang.org/grpc: v1.80.0
  github.com/stretchr/testify: v1.11.1
  # ...high-frequency shared deps only
```

Internal modules (`go-spring.org/...`) are intentionally absent - the workspace
resolves them through `go.work`, so they must never be pinned via `require`.

## Commands

All invoked through `scripts/versions.sh` (which runs `go run ./cmd/bomtool` in
`gs/gs`):

| Command | Effect |
| --- | --- |
| `./scripts/versions.sh check` | Read-only. Prints every module whose require version drifts from the baseline; exits non-zero if any drift, so it can gate a check script. |
| `./scripts/versions.sh diff` | Read-only. Groups deviations by dependency (blessed version + each deviating module) for human remediation decisions. |
| `./scripts/versions.sh apply <module>` | Writes **one** module's `go.mod`, aligning its governed require versions to the baseline. Accepts a module path or its workspace directory. |

`apply` deliberately targets a single module so batch remediation stays serial
and never collides with concurrent work on other modules. After `apply`, run
`go mod tidy` in that module to settle `go.sum` and restore `// indirect`
markers.

## Where it runs

The drift check runs as a step in `scripts/check-go-modules.sh`, the repo's
maintainer check script, so drift surfaces during local checks. This repo has no
CI pipeline of its own; adopters wiring the BOM check into their own CI can call
`./scripts/versions.sh check` directly - it exits non-zero on drift.

> The `disabled` go-versions list in `versions.yaml` is recorded for reference
> but not yet enforced by the check.
