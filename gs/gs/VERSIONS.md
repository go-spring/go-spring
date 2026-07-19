# Dependency Version Governance (BOM)

Go-Spring is a workspace of 60+ independent `go.mod` modules. Without a single
place to align versions, shared dependencies drift apart — and version skew has
already caused real breakage (a go1.26 toolchain suffix crashed codegen tools).

`versions.yaml` at the repo root plays the role Spring's BOM plays: it records
the third-party versions Go-Spring has "blessed", and `gs versions` reports (and
optionally aligns) modules that deviate from that baseline.

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

Internal modules (`go-spring.org/...`) are intentionally absent — the workspace
resolves them through `go.work`, so they must never be pinned via `require`.

## Commands

| Command | Effect |
| --- | --- |
| `gs versions check` | Read-only. Prints every module whose require version drifts from the baseline; exits non-zero if any drift, so it can gate CI. |
| `gs versions diff` | Read-only. Groups deviations by dependency (blessed version + each deviating module) for human remediation decisions. |
| `gs versions apply <module>` | Writes **one** module's `go.mod`, aligning its governed require versions to the baseline. Accepts a module path or its workspace directory. |

`apply` deliberately targets a single module so batch remediation stays serial
and never collides with concurrent work on other modules. After `apply`, run
`gs go mod tidy` in that module to settle `go.sum` and restore `// indirect`
markers.

## CI hook (recommendation)

Run the read-only check in your check/CI script so drift is caught early. It is
**not** wired into this repo's build — adopt it per project as needed:

```sh
# in check.sh / CI pipeline — fail the build on version drift
gs versions check
```

`gs versions check` returns a non-zero exit code when any governed module
deviates from `versions.yaml`, which most CI runners surface as a failed step.
Pair it with a periodic `gs versions diff` review to decide when to bump the
baseline itself.
