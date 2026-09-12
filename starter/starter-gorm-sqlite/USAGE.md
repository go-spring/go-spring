# starter-gorm-sqlite Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`) and the runnable [example/](example/)
(smoke: `example/check.sh`, no docker needed). **SQLite and GORM semantics are their own
docs** — driver: [glebarez/sqlite](https://github.com/glebarez/sqlite) (pure-Go, built on
[modernc.org/sqlite](https://gitlab.com/cznic/sqlite); no cgo), ORM: [gorm.io](https://gorm.io/docs/).
Shared wrapper lifecycle, pool/observe/health wiring and `UseDBCustomizer` live in
[gormcore](../starter-gorm/USAGE.md) and are not repeated here.

**Activation**: one `*gormcore.DB` bean named `sqlite.<entry>` (plus a paired `health.Indicator`) per entry under
`spring.gorm.sqlite`; zero entries → the starter registers nothing and the app starts
(`starter_test.go:TestSqliteDefaultsNotTriggered`). SQLite is in-process: **no TLS block and
no service-discovery path exist for this dialect** (the discovery keys of the shared Common
block are deliberately not bound here — see §3.2).

---

## 1. Complete worked project

A service that keeps a small KV store in a file-backed SQLite database, plus an in-memory
instance for tests. File tree (mirrors [example/](example/)):

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── check.sh        # copies example/check.sh
```

**go.mod** (deps that matter):

```
require (
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-gorm-sqlite latest
    gorm.io/gorm                  v1.31.x
)
```

**main.go**:

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "os"
    "syscall"
    "time"

    "go-spring.org/spring/gs"
    gormcore "go-spring.org/starter-gorm"
    _ "go-spring.org/starter-gorm-sqlite"
)

type greeting struct {
    ID      uint   `gorm:"primaryKey"`
    Message string `gorm:"size:255"`
}

type Service struct {
    DB *gormcore.DB `autowire:"sqlite.primary"` // instance "primary" from conf
}

var manual = flag.Bool("manual", false, "keep the process up")

func main() {
    flag.Parse()
    bean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
    if !*manual {
        go func() {
            time.Sleep(500 * time.Millisecond)
            runTest(bean.Interface().(*Service))
        }()
    }
    gs.Run()
}

func runTest(s *Service) {
    ctx := context.Background()
    var version string
    if err := s.DB.WithContext(ctx).Raw("SELECT sqlite_version()").Scan(&version).Error; err != nil {
        fmt.Println("VERSION failed:", err)
        os.Exit(1)
    }
    if err := s.DB.WithContext(ctx).AutoMigrate(&greeting{}); err != nil { /* ... */ }
    if err := s.DB.WithContext(ctx).Create(&greeting{Message: "hello"}).Error; err != nil { /* ... */ }
    fmt.Println("SQLite round trip OK:", version)
    syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
}
```

**conf/app.properties** — the full surface actually used (copied from
`example/conf/app.properties`):

```properties
# In-memory database; each connection opens its own :memory: store, so pin the
# pool to a single connection for a stable in-memory round trip (see §4.1).
spring.gorm.sqlite.instances.primary.file=:memory:
spring.gorm.sqlite.instances.primary.journal-mode=wal
spring.gorm.sqlite.instances.primary.busy-timeout=5000
spring.gorm.sqlite.instances.primary.foreign-keys=true
spring.gorm.sqlite.instances.primary.max-open-conns=1

# A file-backed database shows the more typical multi-connection setup.
# spring.gorm.sqlite.instances.file.file=/tmp/go-spring-example.db
# spring.gorm.sqlite.instances.file.max-open-conns=8
```

**Verify** (identical in shape to `example/check.sh`):

```bash
go run . > smoke.out 2>&1 &
# ... wait, then:
grep "SQLite round trip OK:" smoke.out    # must print; exit code alone is not proof
```

External dependencies: **none** — SQLite is in-process and the driver is pure Go (no cgo,
cross-compiles anywhere). This is the only gorm dialect starter whose smoke test runs
without docker.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-gorm-sqlite
  └─ init(): gormcore.Module(Dialect{Prefix: "spring.gorm.sqlite", ...})
        │
gs.Run()
  ├─ OnProperty("spring.gorm.sqlite") fires only if ≥1 entry exists
  ├─ conf.BindEach: each entry → Config (value tags; file passes expr "$ != ''")
  ├─ build(): Config → Spec{Dialector: glebarez/sqlite.Open(DSN), Pool, Resource,
  │          "gorm:sqlite"+file, ObserveEnabled}          — starter.go:49-56
  ├─ gormcore.Open: gorm.Open → ApplyPool (startup ping, PingTimeout bound)
  │                 → DBCustomizers → *DB wrapper
  ├─ gs field-injects Observability → DB.Init(): observe plugin (db.system=sqlite)
  │                 + resilience/fault executor callbacks (resource "gorm:sqlite")
  ├─ run; readiness folds in the gorm:sqlite:<name> PING indicator
  └─ SIGTERM → DB.Destroy(): executor close → closers (none for sqlite) → pool close
```

No TLS registration (no transport) and no discovery dialer (the "server" is a file path) —
`starter.go:46-48` states this explicitly.

### 2.2 One query, layer by layer

`db.WithContext(ctx).First(&g, 1)`:

1. gorm's `gorm:query` processor runs — but gormcore's `ApplyCallbacks` has *replaced* it
   with a wrapper that runs the op under the instance's resilience executor (timeout /
   retry / breaker via the governance center, fault injector when armed). `gorm.ErrRecordNotFound`
   is treated as success so "no rows" never trips the breaker
   (`starter-gorm/resilience/callbacks.go:runGuard`).
2. the observe plugin's `before_query` opens a span + in-flight metric
   (`starter-gorm/observe/plugin.go`; SQL is not known yet).
3. the original processor executes: pool checkout (SQLite: the single `:memory:` conn, or
   one of N file-backed conns — each conn re-applies the `_pragma` settings from the DSN),
   statement build, rows scan.
4. `after_query` sets the SQL on the span, ends span + duration metric, writes the access
   log (level from the wrapper `observability` key).
5. the executor wrapper propagates rejections (`ErrCircuitOpen` etc.) onto `tx.Error`.

---

## 3. Per-key behavior reference

Keys under `spring.gorm.sqlite.instances.<name>.*`. Pool keys (6, `gormcore.PoolSettings`),
`observe.enabled` and the
wrapper `observability` key are shared — see
[gormcore](../starter-gorm/USAGE.md#2-configuration-reference).

### 3.1 SQLite-specific keys (config.go:29-49)

| Key | Type | Default | Behavior / coupling | Misconfiguration consequence |
|-----|------|---------|---------------------|------------------------------|
| `file` | string | — **required** (expr `$ != ''`) | SQLite path or `:memory:`; sole input to DSN(). | Empty → bind-time expr failure, instance (and app) fails to start. |
| `journal-mode` | string | `wal` | Appended as `_pragma=journal_mode(wal)`. Values: wal/delete/truncate/persist/memory/off (SQLite semantics — [pragma docs](https://sqlite.org/pragma.html#pragma_journal_mode)). Empty string omits the pragma entirely (config.go:56). | `memory`/`off` weaken durability; invalid mode → SQLite falls back with a warning, data still works. |
| `busy-timeout` | int (ms) | 5000 | `_pragma=busy_timeout(5000)`; concurrent writers wait instead of failing `SQLITE_BUSY`. `0` omits the pragma. | Too low + concurrent writers → `database is locked` errors at query time, not at boot. |
| `foreign-keys` | bool | true | `_pragma=foreign_keys(1)` when true; **false omits the pragma**, it does not emit `foreign_keys(0)` (config.go:62-64). ⚠ per-connection: applies to every pool conn via DSN. | Expecting FK enforcement with `foreign-keys=false` → constraints silently not enforced (SQLite default is off). |
| `max-open-conns` (shared pool key) | int | 0 | ⚠ With `file=:memory:` this MUST be `1` — see §4.1. | Missing → intermittent "table not found" / lost writes. |

### 3.2 Removed keys

`service-name`, `scheme`, `discovery` were previously bound via the embedded
`gormcore.Common` but never read by the sqlite `build` — there is no server to discover.
They were **removed** (2026-08): the sqlite Config now embeds only `gormcore.PoolSettings`
(pool + ping + slow-threshold) plus `observe.enabled`. Setting them now fails with an
unknown-key error instead of being silently ignored.

### 3.3 Keys that do not exist for this dialect

`tls.*` — there is no TLS block on the sqlite Config at all (unlike sqlserver/clickhouse);
transport encryption is a filesystem-permissions concern.

---

## 4. Verification & fault drills

### 4.1 Drill: the `:memory:` concurrency trap

Why it exists: with `file=:memory:` every pool connection opens its **own private memory
database** (SQLite semantics, not a driver choice). With `max-open-conns` unset (0 =
unlimited) the AutoMigrate may run on conn A while the insert runs on conn B →
`no such table`.

Demonstrate (same shape as the example, which pins `max-open-conns=1` in
`example/conf/app.properties:3-8`):

```properties
# spring.gorm.sqlite.instances.primary.max-open-conns=   ← comment this out to reproduce
spring.gorm.sqlite.instances.primary.file=:memory:
```

```bash
go run .   # intermittently fails "no such table: greetings"
# restore max-open-conns=1:
go run .   # prints "SQLite round trip OK:"
```

Escapes if you need concurrency in memory: a **file** db (`file=/tmp/x.db`, optionally on a
tmpfs), or `file=file::memory:?cache=shared` written by hand via `UseDBCustomizer` (the
DSN builder does not add `cache=shared` for you).

### 4.2 Verify the wiring

```bash
grep "SQLite round trip OK:" smoke.out     # CRUD + transaction round trip
curl -s :9370/readyz                        # with starter-actuator: gorm:sqlite:<name> folded in
```

Per-query observables (when `observe.enabled=true`, the default): span
`db.system=sqlite` per Create/Query/Update/Delete, duration/in-flight metrics, access log
with the SQL statement. Kill it per instance with `observe.enabled=false` (plugin not
installed at all).

### 4.3 Drill: busy-timeout under writer contention

Two instances against one file db with `busy-timeout=1`, then a deliberate long write
transaction in one — the other waits ~1 ms then fails `SQLITE_BUSY`. Raise to the default
5000 and retry.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| "no such table" on `:memory:` | pool > 1 conn, each with its own memory db | `max-open-conns=1` (mandatory for `:memory:`). |
| `database is locked` under concurrent writers | `busy-timeout` too low / journal-mode not wal | keep defaults (`busy-timeout=5000`, `journal-mode=wal`). |
| Instance fails to start: expr `$ != ''` | `file` empty/missing | set `file` — it is the only required key. |
| FK constraint not enforced | `foreign-keys=false` (pragma omitted, SQLite default off) | set `true`. |
| Second instance sees none of the first's data | file-backed paths differ / relative path depends on cwd | use absolute paths; the example `init()` chdirs to the source dir. |
| No spans/metrics per query | `observe.enabled=false` or starter-otel not imported | re-enable / import starter-otel (hooks are silent no-ops without it). |
| Slow-query log entries look odd | `slow-threshold>0` routes GORM's warn-level output through `go-spring.org/log` (TagAppDef) since 2026-08 | output is GORM's one-line text; filter by message if noisy. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Dialect-specific keys | 4 (+6 shared pool keys, +1 `observe.enabled`, +1 wrapper `observability`) |
| Required | 1 (`file`) |
| Quickstart external deps | 0 (only gorm dialect starter with none) |
| Dead bound keys | 0 (discovery keys removed 2026-08) |
| "Watch out" entries | 4 |

Design suspects: `foreign-keys=false` *omits* rather than negates the pragma (asymmetric knob); DSN builder
cannot express key-only pragmas or `file::memory:?cache=shared` (documented in
config.go:50-53); the `:memory:`-vs-pool trap is config-invisible — no startup guard
refuses `:memory:` with `max-open-conns != 1`.
