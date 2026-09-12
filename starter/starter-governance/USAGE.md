# starter-governance Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against
the starter source (`starter.go`, `wiring.go`, `source_file.go`, `source_http.go`, `rules/rules.go`,
`wiring_test.go`, `source_file_test.go`, `source_http_test.go`), the core package it wires
(`cloud/governance`: `govern.go`, `source.go`, `global.go`, `fault/config.go`,
`resilience/config.go`), and the runnable [example/](example/) (`example/main.go` prints the
resolved policy + fault config every second, prompts you to edit the rules file live, and
self-terminates after 6s unless `-manual`).

**What this starter is**: the wiring between gs and the container-free governance core
(`cloud/governance`). Blank-importing it is inert until configured. Two roles:

1. **Wiring** (always registered, `wiring.go`): hands the injected `governance.Source` bean to
   the governance center, registers the executor/fault seams and marks the authority live.
2. **Source adapters** (conditional, `source_file.go` / `source_http.go`): when a
   `govern.source.*` key is present, a `governance.Source` bean is injected into the wiring;
   rules refresh governance only — never an app-wide property re-bind.

Governance configuration is its own document (a rules file, a console, a config center) — it is
NOT written into `app.properties`; only the one `govern.source.*` bootstrap key is.

Governance semantics themselves (policy resolution, breaker/retry/ratelimit behavior, fault
injection model) are documented in `cloud/governance` — everything below is the starter's wiring
increment plus the contract you need to use it.

---

## 1. Complete worked project

A minimal service whose governance rules live in their OWN file and hot-reload live. File tree
(this is the checked-in example, verbatim):

```
demo/
├── go.mod
├── main.go
└── conf/
    ├── app.properties
    └── govern.yaml
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-governance latest
)
```

**main.go**:

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "time"

    "go-spring.org/cloud/governance"
    "go-spring.org/cloud/governance/fault"
    "go-spring.org/spring/gs"

    _ "go-spring.org/starter-governance"
)

// printer prints the resolved policy AND the fault-injection config for one
// resource label every second — resilience and fault ride the same source.
type printer struct{}

func (p *printer) Run(ctx context.Context) error {
    tk := time.NewTicker(time.Second)
    defer tk.Stop()
    for i := 0; ; i++ {
        p := governance.PolicyFor("demo:resource")
        fmt.Printf("policy: enabled=%v timeout=%v retries=%d rate-limit=%v",
            !p.IsZero(), p.Timeout, p.MaxRetries, p.RateLimit)
        if in := fault.InjectorFor(); in != nil {
            c := in.Config()
            fmt.Printf(" | fault: enabled=%v rate=%v", c.Enabled, c.Rate)
        }
        fmt.Println()
        if i == 3 {
            fmt.Println(">>> edit conf/govern.yaml now: policy AND fault toggle live")
        }
        select {
        case <-ctx.Done():
            return nil
        case <-tk.C:
        }
    }
}

func init() {
    gs.Provide(&printer{}).Export(gs.As[gs.Runner]())
}

func main() {
    manual := flag.Bool("manual", false, "run in manual verification mode (stay up until killed)")
    flag.Parse()
    go func() {
        time.Sleep(6 * time.Second) // give the operator time to edit, then exit
        _ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
    }()
    gs.Run()
}
```

**conf/app.properties** — one line is the entire wiring:

```properties
# The governance rules live in their OWN file, watched by starter-governance's
# file source — nothing under govern.* here. This key arms the starter's
# conditional module, whose Source bean is injected onto the governance center.
govern.source.file.path=conf/govern.yaml
```

**conf/govern.yaml** — same keys an app.properties entry would use, watched live:

```yaml
govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
    max-retries: 2
  fault:
    enabled: false        # flip to true mid-run — see §4.2
    rate: 0.5
  rules:
    - resources: demo:resource
      attempt-timeout: 50ms
```

**Verify** (from the example directory; the app self-terminates after 6s unless `-manual`):

```bash
go run . -manual
# policy: enabled=true timeout=100ms retries=2 rate-limit=0 | fault: enabled=false rate=0.5
# >>> edit conf/govern.yaml now: policy AND fault toggle live
#   ... change attempt-timeout to 77ms, or fault.enabled to true ...
# policy: enabled=true timeout=77ms retries=2 rate-limit=0 | fault: enabled=true rate=0.5
```

The change lands within ~1s of saving (fsnotify), with no restart and no app-wide re-bind.

**Variant — another source**: swap `govern.source.file.path` for `govern.source.http.*` (a
console), or a nacos/etcd source key from their own starters. Exactly one source is active per
process, and there is no longer an app.properties path: rules always come through a Source.

**Variant — combining with a server starter**: any client starter wired to the neutral seams
(`resilience.ExecutorFor(system, label)`, `fault.InjectorFor()`) picks the policy up automatically — no
application code changes. See starter-echo's USAGE §4.4 for a worked fault drill through an HTTP
server with `scope: loadtest`.

---

## 2. Assembly & timing

### 2.1 The Source contract — what the center actually depends on

`cloud/governance` is container-free: its `center` depends only on the two-method `Source`
interface (`cloud/governance/source.go:43`):

```go
type Source interface {
    Snapshot() Config                       // latest committed value; zero Config before any push
    Subscribe(cb func(Config))              // invoked with each new config after it commits
}
```

Deliberate omissions (from the interface's doc comment):

- **No error returns** — the center cannot roll back a bad push. "Everything you push, you vouch
  for": keeping the last good snapshot on a bad document is the source's concern (both adapters
  here do exactly that).
- **No Close in the interface** — lifecycle belongs to the implementation; the center type-asserts
  `interface{ Close() error }` at Destroy and closes sources that happen to implement it.
- **A single callback** — the center is the only consumer; a second Subscribe may replace the
  first.

The wiring picks its source from the bean injected into `wiring.Src` (`wiring.go:38-43`). The
adapter family: `FileSource` (fsnotify), `HTTPSource` (interval poll) in this starter;
nacos (ListenConfig push) in `experimental/starter-config-nacos`, etcd (Watch push) in
`starter-config-etcd`; `governance.PushSource` for hand-rolled push integrations.

### 2.2 Bean lifecycle timeline

```
import starter-governance
  ├─ init() wiring.go: gs.Provide(newWiring)
  │      .Init((*wiring).Init).Destroy((*wiring).Destroy)
  │      .Export(gs.As[gs.Rooter]())            ← the wall + the fix, see below
  └─ init() starter.go: two conditional gs.Module beans
         ├─ OnProperty("govern.source.file")  → *FileSource, Export(As[governance.Source]())
         └─ OnProperty("govern.source.http")  → *HTTPSource, Export(As[governance.Source]())
              (OnProperty is a PREFIX check: any govern.source.<x>.* key arms it)

gs.Run()
  ├─ config bind: source config from ${govern.source.file|http.*} (expr-validated)
  ├─ bean wiring: the source bean's Export makes it visible; it is field-injected into
  │      wiring.Src (`autowire:"?"` — nullable: no bean ⇒ nil ⇒ default path)
  ├─ wiring.Init():
  │      BindDefault(Src)   (nil-safe: no source bean ⇒ stays disabled)
  │        ├─ explicit SetSource already called?  → BindDefault is a NO-OP (SetSource wins)
  │        ├─ bindSource: subscribe (stale-guarded by handle pointer) + adopt Snapshot()
  │      GoLive(): build process-wide *fault.Injector from the snapshot,
  │        resilience.RegisterExecutorProvider, fault.RegisterInjector, markLive()
  │        (→ fires every queued OnReady callback)
  ├─ source bean Init: Start() — fsnotify watch / poll loop begins
  ├─ your Runners run (policy already armed — Rooter precedes Runner)
  └─ on SIGTERM: wiring.Destroy() → governance.CloseActiveSource()
        (closes the source iff it implements Close; FileSource stops its watcher,
         HTTPSource cancels its poll loop)
```

Two design points worth knowing (both from source comments, verified):

- **The Rooter export is load-bearing.** The `wiring` bean is not injected by anything, and gs
  does not instantiate beans that are neither exported nor injected — without
  `Export(gs.As[gs.Rooter]())` none of the registrations would fire in production (`wiring.go:53-58`).
  Similarly, a custom Source bean of yours is invisible to interface injection unless you
  `Export(gs.As[governance.Source]())` it — a missing Export silently leaves governance disabled.
- **The Center never enters the container.** `cloud/governance` exposes only package-level
  functions; the singleton (`global.go:42`) is armed in place by the wiring bean. Nothing outside
  the package obtains a `*center`.

### 2.3 Rooter vs Runner, and why OnReady exists

gs collects and runs `gs.Rooter` beans before `gs.Runner` beans. The wiring bean is a Rooter, so
governance is armed before your Runners start — the common case needs nothing. But a push-based
caller that is *itself* a Rooter (e.g. starter-dubbo's poller) may initialize before the wiring
Rooter has armed governance. `governance.OnReady(cb)` (`global.go:145-158`) solves this without
depending on bean order: cb queues until the authority goes live, then fires exactly once; if it
is already live, cb fires immediately. The double-checked locking guarantees each callback runs
exactly once whichever side wins the race.

### 2.4 One rules edit, end to end

1. You save `conf/govern.yaml` (any editor; atomic rename is fine — the watcher watches the
   parent *directory*, never the file, precisely because rename swaps the inode
   (`source_file.go:87-94`). The loop reacts to every directory event, not just the file's name.)
2. `FileSource.reload()` re-reads and parses through `rules.Parse` — format inferred from the
   extension (`.yaml`), parsed by the shared conf reader registry, flattened, required to carry
   at least one `govern.*` key, then bound into `governance.Config` through the same value-tag
   machinery (`rules/rules.go:57-75`). A parse failure or a
   govern-key-less document logs `reload ... failed (keeping last good config)` and stops.
3. Unchanged config (DeepEqual) pushes nothing — a touch does not churn executors.
4. The center's subscribed callback adopts the config: `refresh(cfg)` stores the atomic snapshot,
   re-resolves the policy for every registered label, and notifies only subscribers whose policy
   actually changed (`govern.go:376-397`); `injector.SetConfig(cfg.Fault)` hot-swaps fault in place.
5. Seam effect: client starters never import governance — they call
   `resilience.ExecutorFor(system, label)` and `fault.InjectorFor()`, both resolved lazily at call time.
   The executor's Refresh swaps the live policy; the fault injector's config is swapped in place,
   so `fault.enabled: true` takes effect on the very next call with no restart.

---

## 3. Per-key behavior reference

### 3.1 Source keys (this starter)

Bound with the explicit prefix `${govern.source.file:=}` / `${govern.source.http:=}` via
`conf.Bind` — these are the only keys under `govern.*` that are wiring, not rules. ⚠ One
namespace, two roles: `govern.*` is the rules; `govern.source.*` is where the rules come from.

| Key | Type | Default | Required | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|----------|-------------------------|------------------------------|
| `govern.source.file.path` | string | — | yes (`expr:"$ != ''"`) | fsnotify on the parent directory; format by extension (json/properties/yaml/toml). Initial load failure fails startup. | Wrong/missing path → startup error (deliberate: a disabled center must not arm silently). |
| `govern.source.http.url` | string | — | yes (`expr:"$ != ''"`) | Polled with GET; document byte-compatible with the file source. | Bad URL / non-200 / bad body → startup error on first fetch; later, keep-last-good + log. |
| `govern.source.http.interval` | duration | 5s | no | Poll period AND the HTTP client timeout (`source_http.go:70`). | Too small → fetches time out against themselves; too large → slow convergence. |
| `govern.source.http.format` | string | inferred from URL extension | no | `"yaml" \| "json" \| "properties" \| "toml"`; overrides inference (use it when the URL has no/useful extension). | Wrong format → parse error → keep-last-good (or startup failure on first fetch). |
| `govern.source.http.headers` | map[string]string | empty | no | Sent with every request, e.g. `headers.authorization=Bearer xxx` for a console. | Missing auth → every fetch 401 → stuck on last good config. |

Exactly ONE source is active per process: configuring both file and http arms two beans but the
center holds one — the bean injection race is unspecified; configure exactly one. Remote adapters
in their own starters: `govern.source.nacos.*` (starter-config-nacos, ListenConfig push) and
`govern.source.etcd.*` (starter-config-etcd: `endpoint`, `key`, …, Watch push) — the documents are
byte-portable across all of these backends.

There is no default path: without a `govern.source.*` key (or an injected/`SetSource` source) the
center stays disabled. ⚠ `govern.source.*` is bootstrap ONLY — the rules themselves never live in
app.properties.

### 3.2 Rules document — top level

| Key | Type | Default | Behavior | Misconfiguration consequence |
|-----|------|---------|----------|------------------------------|
| `govern.enabled` | bool | false | Master switch. false → PolicyFor always returns a zero Policy (pass-through) regardless of Default/Rules. | Everything configured but still off — the #1 "why doesn't it work" cause. |
| `govern.driver` | string | "default" | Resilience backend for ALL resources: "default" or "sentinel". | Unknown driver → executor build fails → no-op executor fallback. |
| `govern.default.*` | PolicyConfig | all off | Baseline policy for every resource no Rule matches. | — |
| `govern.rules[n].*` | []Rule | empty | First Rule whose Resources contains the label wins. ⚠ A matched Rule **fully replaces** Default — no field-wise merge: a zero policy field means "disabled", so a partial merge could not distinguish "explicitly 0" from "unset" (`govern.go:87-90`). List specific rules first. | Rule setting only `attempt-timeout` silently turns OFF the default's retries for that resource. |
| `govern.fault.*` | fault.Config | all off | Process-wide fault injection, see §3.4. | — |

Resource labels live in a **value**, never a key (`govern.go:96-100`): `redis:cache`,
`gorm:mysql:primary`, `gin:api`, `dubbo:com.example.Foo:1.0.0` — colons and dots need no escaping.

### 3.3 Policy knobs (16, usable under `govern.default.*` and each `govern.rules[n].*`, all default 0/off)

| Key | Type | Group | Notes |
|-----|------|-------|-------|
| `rate-limit` / `burst` | float / int | ratelimit | ops/sec cap; burst defaults to a small multiple of rate-limit when unset. |
| `error-threshold` / `open-duration` | int / duration | breaker | consecutive-error trip. |
| `breaker-strategy` | enum | breaker | `consecutive` \| `error-rate`. |
| `error-rate-threshold` / `min-requests` / `breaker-window` | float / int / duration | breaker | error-rate strategy inputs. |
| `max-concurrent` | int | isolate | concurrent-call cap. |
| `max-retries` / `initial-interval` / `multiplier` / `max-interval` / `randomization-factor` | … | retry | exponential backoff family. |
| `attempt-timeout` / `max-duration` | duration / duration | timeout | per-attempt bound; overall bound (attempt budget = min(Timeout, remaining MaxDuration)). |

Driver selection and the on/off switch are process-wide (`govern.enabled`/`govern.driver`) and
deliberately NOT re-bindable per resource. Semantics of each knob (backoff math, breaker states)
are `cloud/governance/resilience` territory — the starter only binds and fans them out.

### 3.4 Fault knobs (`govern.fault.*`)

| Key | Type | Default | Behavior |
|-----|------|---------|----------|
| `enabled` | bool | false | When false nothing is injected even if rate/error are set. |
| `rate` | float | 0 | Per-call probability of injecting the configured error (0..1). |
| `latency` | duration | 0 | Injected before each call and before the error decision, on EVERY call regardless of rate — models a uniformly slow downstream without forcing errors. |
| `error` | string | "" | `""`/`generic` (retryable ErrInjected) \| `timeout` (wraps DeadlineExceeded) \| `reset` (wraps ECONNRESET). Empty + rate 0 injects nothing. |
| `scope` | string | "" | `""` all traffic \| `real` only unmarked \| `loadtest` only traffic carrying the load-test marker (X-LoadTest via the traffic middleware). Unknown value behaves as "". |
| `max-duration` | duration | 0 | Safety auto-off: after this long since the first affected call, injection stops — a forgotten fire self-heals. ⚠ Hot-reload: shortening takes effect promptly; lengthening does NOT un-trip an already-expired fire (re-arm by toggling enabled off→on). |
| `max-affected` | int64 | 0 | Blast-radius cap: stop after this many faulted calls. |
| `rules[n].resources` / `.rate` / `.latency` / `.error` | … | empty | Per-resource overrides; FIRST matching rule wins. ⚠ Asymmetry vs resilience: a fault rule with EMPTY resources is a catch-all; when no rule matches, the top-level values still apply — adding rules only adds specificity. |

---

## 4. Verification & fault drills

### 4.1 Watch the hot reload (example app)

```bash
cd example && go run . -manual
# baseline:  policy: enabled=true timeout=100ms retries=2 rate-limit=0 | fault: enabled=false rate=0.5
sed -i '' 's/attempt-timeout: 100ms/attempt-timeout: 77ms/' conf/govern.yaml   # lands within ~1s
```

Negative drill — bad edit keeps the last good config (watch the log tag `governance`):

```bash
echo "govern: {enabled: true}" > conf/govern.yaml        # truncated: still has a govern key
printf '' > conf/govern.yaml                              # EMPTY: no govern.* key → reload error
# log: governance file source: reload conf/govern.yaml failed (keeping last good config): ...
```

Turning governance off is `govern.enabled=false` — a key that IS present — never an emptied file.

### 4.2 Fault drill, no restart

1. Start with `fault.enabled: false` in govern.yaml.
2. Flip it to true and save — `injector.SetConfig` swaps in place; the next call is subject to it.
3. With `scope: loadtest`, only traffic marked `X-LoadTest: 1` burns; `real` inverts that
   (dedicated environments only); empty scope hits everything.
4. Add the guards before walking away: `max-duration: 5m` (auto-off) and
   `max-affected: 1000` (blast-radius cap).
5. Extinguish by flipping `enabled` back to false (or wait out max-duration).

### 4.3 Probing the facade from your own code

```go
if governance.Enabled() { ... }                    // armed? (false before live)
p := governance.PolicyFor("redis:cache")           // zero Policy = pass-through
governance.Register("redis:cache", func(p resilience.Policy) { /* re-arm client */ })
governance.OnReady(func() { /* runs once when the authority goes live */ })
in := fault.InjectorFor(); c := in.Config()        // read live fault config (nil when absent)
```

### 4.4 Testing API (non-gs path)

`governance.Arm(cfg)` installs a center built from cfg onto the singleton, marks it live and
returns a reset func; `governance.Reset()` restores the disabled default. ⚠ `gs.RunTest` does NOT
exercise this path: gs clones global bean definitions for test isolation, so the test app wires a
*copy* of the wiring bean while the facade still reads the package singleton — the two never meet.
This starter's own tests drive `wiring.Init()` directly instead (`wiring_test.go:53-56`). In app
tests, use `Arm`/`Reset`, or call the facade directly; do not expect RunTest-registered beans to
change what `governance.PolicyFor` sees.

### 4.5 Push-based custom source

```go
src := governance.NewPushSource(governance.Config{})
governance.SetSource(src)   // any time: before wiring (pre-empts the default) or after (late-arm)
// on each upstream event:
src.Push(cfg)
```

`SetSource` outranks the injected bean and the default; exactly one source is active at a time;
the center never merges — whole-replace; removing a custom source is not supported (restart).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Everything configured, nothing governed | `govern.enabled` false (default) | Set `govern.enabled=true` — it is the master switch. |
| Rules file edited, policy never changes | Wrong path; or the file is a symlink target the dir-watch misses (rare) | Check the startup-armed path; check the log tag `governance` for `reload ... failed`. |
| `reload ... failed (keeping last good config)` in logs | Truncated/emptied document (no `govern.*` key) or syntax error | Fix the document; "off" is `govern.enabled=false`, not an empty file. |
| Startup error `governance source: ... contains no govern.* keys` | First load of an empty document | Same as above, at construction time — deliberate. |
| Custom Source bean silently ignored | Missing `Export(gs.As[governance.Source]())` | Add the Export — without it the bean is invisible to interface injection. |
| HTTP source stuck on old rules | Console auth/availability: every poll fails, keep-last-good | Check `headers.*`; check the console returns 200 with a valid document. |
| Per-resource rule killed the default's retries | Matched rule fully REPLACES default (no merge) | Restate every knob you want kept in the rule. |
| Fault `max-duration` raised but fire stays out | Lengthening does not un-trip an expired fire | Toggle `fault.enabled` off→on to re-arm. |
| RunTest asserts on governance, sees disabled center | RunTest clones beans; the facade reads the package singleton | Use `governance.Arm`/`Reset` in tests instead. |
| A Rooter bean starts before governance is armed | Rooter-vs-Rooter order is unspecified | Wrap the dependent work in `governance.OnReady`. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys (source) | 5 (this starter) + nacos/etcd in their starters |
| Required | 1 (the chosen source's `path`/`url`) |
| Rules-document keys | 4 top + 16 policy knobs ×2 (default/rules[n]) + 7 fault + 4 fault-rule |
| Quickstart external deps | 0 (file source); a console for http |
| "Watch out" entries (⚠ above) | 7 |

Design suspects (audit ledger; carried over from the previous edition, none newly fixed):

1. `govern.*` is both the rules namespace and the source config lives under `govern.source.*` —
   one namespace, two roles.
2. Replace-vs-merge asymmetry between resilience rules (full replace, empty-resources matches
   nothing) and fault rules (catch-all + global fallback) — must be memorized.
3. A custom Source bean without `Export(gs.As[...])` silently leaves governance disabled (startup
   leaves the center disabled).
5. The example's self-kill smoke script is still not checked in as a CI-runnable `check.sh`.
6. Concepts the doc must define (Center / Source / Snapshot vs Subscribe / seams / adopt) — the
   mental model is real work for a first-time user.
7. ~~Rooter beans may initialize before governance arms, with no ordering guarantee~~ — FIXED:
   `governance.OnReady` queues-and-fires exactly once at `GoLive` (`global.go:145`).
8. ~~Fault injection required a restart to toggle~~ — FIXED: `injector.SetConfig` swaps in place
   on every source push (`govern.go:209-214`).
