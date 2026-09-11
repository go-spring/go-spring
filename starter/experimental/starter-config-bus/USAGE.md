# starter-config-bus Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `bus.go`, `config.go`) and the runnable,
docker-gated [example/](example/) (`example/check.sh` smoke test). The bus is a config
**event bus**: it carries refresh *signals* over NATS, never configuration content — the
source of truth stays with your config center or local files. NATS's own semantics are
[nats.go documentation](https://docs.nats.io/); everything below is go-spring's increment.

> `experimental/` marker: this starter is **unreviewed** — it has not passed a design audit.

**Activation**: blank-importing the package registers the `ConfigBus` bean under an
`OnProperty("spring.config.bus")` condition (`starter.go:49-58`). With **no**
`spring.config.bus.*` property set, the starter assembles nothing and the import is inert —
you can ship the import in a shared module without every app configuring NATS. Once any
`spring.config.bus.*` key is present (setting `spring.config.bus.subject` alone is enough),
the bean assembles and the autowire tag
`${spring.config.bus.nats-instance:=config-bus}` (`bus.go:55`) requires a NATS instance named
`config-bus` (or your override) to exist under `spring.nats.*`; if it does not, container
wiring fails and startup aborts. There is no separate `enabled` switch.

> **Migration note (behavior change):** before the condition gate, blank-importing this
> package with zero NATS instances failed startup with a missing-bean wiring error. Now the
> same import starts cleanly and the bus stays dormant until `spring.config.bus.*` appears.
> If you relied on that startup failure as a misconfiguration alarm, assert the presence of
> `spring.config.bus.*` in your own startup checks instead.

---

## 1. Complete worked project

A service whose dynamic config refreshes fleet-wide when one instance publishes a refresh
signal. File tree (isomorphic to the smoke-tested [example/](example/)):

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── docker-compose.yml      # local NATS broker
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-config-bus latest
    go-spring.org/starter-nats       latest   // transport provider — required
)
```

**main.go**:

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "go-spring.org/log"
    "go-spring.org/spring/gs"

    StarterConfigBus "go-spring.org/starter-config-bus"
    _ "go-spring.org/starter-nats"
)

// Demo binds a dynamic field and holds the bus so it can broadcast a refresh.
// Export as gs.Rooter so the container creates it eagerly even though nothing
// else injects it (see §2.1).
type Demo struct {
    Bus     *StarterConfigBus.ConfigBus `autowire:"configBus"` // bean name from starter.go
    Message gs.Dync[string]             `value:"${demo.message:=none}"`
}

func main() {
    gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())

    // Stand in for "something changed that the config center's own watch did
    // not observe": raise a value on an env source, then broadcast once.
    go func() {
        time.Sleep(500 * time.Millisecond)
        want := "v-" + time.Now().Format("150405")
        _ = os.Setenv("GS_DEMO_MESSAGE", want) // env source → demo.message
        if err := getDemo().Bus.Publish(""); err != nil { // "" = full-fleet refresh
            log.Errorf(context.Background(), log.TagAppDef, "publish failed: %v", err)
            os.Exit(1)
        }
        deadline := time.Now().Add(15 * time.Second)
        for time.Now().Before(deadline) {
            if getDemo().Message.Value() == want {
                fmt.Println("config-bus refresh observed:", want)
                return // field flipped without a restart
            }
            time.Sleep(200 * time.Millisecond)
        }
        os.Exit(1)
    }()
    gs.Run()
}
```

(In the checked-in example the bean handle is captured directly from `gs.Provide`; the
structure above is the same flow. See [example/example.go](example/example.go).)

**conf/app.properties** — the complete, commented surface:

```properties
# --- NATS transport (starter-nats instance named "config-bus") ---------------
# The name MUST match spring.config.bus.nats-instance (default "config-bus").
spring.nats.config-bus.url=nats://127.0.0.1:4222

# --- config bus ---------------------------------------------------------------
# Subject: all instances sharing it form one bus. Defaults shown.
# spring.config.bus.subject=spring.config.refresh
# spring.config.bus.watch-prefixes=db,cache

# --- the dynamic value --------------------------------------------------------
demo.message=v0
```

**docker-compose.yml**:

```yaml
services:
  nats:
    image: nats:2.10
    ports:
      - "127.0.0.1:4222:4222"
```

**Verify** (same shape as `example/check.sh`):

```bash
docker compose up -d
go run .              # expect: "subscribed to bus subject=..." then
                      # "config-bus refresh observed: v-..." and clean exit
docker compose down -v
```

Manual mode (`go run . -manual` in the example) keeps the process up so you can publish
from a second terminal or observe a second instance refreshing.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle — when the bus assembles and why

```
import starter-config-bus
  └─ init(): gs.Provide(&ConfigBus{}).Name("configBus")
              .Init(subscribe).Destroy(Destroy).Export(gs.As[gs.Rooter]())   [starter.go:44-54]

gs.Run() → App.Start()                                                       [gs_app/app.go]
  ├─ mount the gs.RefreshProperties / gs.AppStarted facade targets
  ├─ property load (all sources) + log init
  ├─ container refresh (bean wiring):
  │    ├─ ConfigBus fields autowired:
  │    │    Conn      ← NATS instance by name (${spring.config.bus.nats-instance:=config-bus})
  │    │    Config    ← ${spring.config.bus} value tags (config.go)
  │    └─ bean Init hook: subscribe() — NATS Subscribe on Config.Subject      [bus.go:66-99]
  │         logs "subscribed to bus subject=... prefixes=[...]"
  ├─ app.started = true            ← RefreshProperties is allowed from here on
  ├─ Runners → Servers → readiness
  └─ on SIGTERM: bean Destroy → sub.Unsubscribe(); the NATS conn closes in starter-nats
```

Two timing facts matter for a *bus*:

- **The Rooter export is what makes it always wire.** The bean has no dependents by default;
  `Export(gs.As[gs.Rooter]())` makes it root-reachable so the container creates it and runs
  `subscribe()` in every process that imports the package — exactly what a fleet-wide bus
  needs (no "forgot to inject it here" instance that silently misses refreshes). The comment
  at starter.go:46-48 states this intent.
- **Init runs before `app.started = true`.** The subscription is live *before* refreshes are
  legal: `RefreshProperties` refuses with "app not started yet" until the flag is set
  (`app.go:247-253`, set at `app.go:305`). A signal arriving in that startup window is logged
  as `config bus: property refresh failed: app not started yet` and otherwise ignored — no
  retry, no queue. In practice the window is milliseconds; know it exists when racing tests.

### 2.2 The startup contract: config-key gate, and no self-hosted keys

- **OnProperty gate.** The bean registers under `Condition(gs.OnProperty("spring.config.bus"))`,
  matching the family convention (config-key-conditional registration). No `spring.config.bus.*`
  key → no bean → inert import. With the key present, the remaining implicit condition is the
  Conn autowire: no `spring.nats.config-bus.*` definition → missing-bean wiring failure at
  startup.
- **Keys the bus refreshes cannot gate the bus itself.** The bus's own keys
  (`spring.config.bus.*`, `spring.nats.<name>.*`) are read once at wiring time. If a refresh
  event changes `spring.config.bus.subject`, the subscription does **not** move — the bean
  re-reads only via a restart. The same applies to `watch-prefixes`: `subscribe()` parses
  them once (`bus.go:67-71`). So treat all `spring.config.bus.*` keys as boot-time constants;
  never put them under a namespace you expect to hot-reload.

### 2.3 Publish → subscribe → gs.Dync refresh, step by step

1. Something changes a config source (config-center push missed by the local watch, a file
   edited on a mount, an env source raised). The bus does not know or care — it is told.
2. Application code (or a management endpoint) calls `bus.Publish(prefix)` — signature
   `func (b *ConfigBus) Publish(prefix string) error` (`bus.go:123`). It marshals
   `RefreshEvent{Prefix: prefix}` to JSON and NATS-publishes to `Config.Subject`.
3. Every instance subscribed to that subject receives the message; each runs the callback
   from `subscribe()` (`bus.go:73-92`):
   - empty payload → treated as a zero-value `RefreshEvent` (full-fleet refresh);
   - malformed JSON → warn `ignoring malformed refresh event` and drop (`bus.go:76-80`);
   - `shouldRefresh(ev.Prefix)` filter (`bus.go:106-116`): honor when the event prefix is
     empty, the instance watches nothing, or the event prefix overlaps a watched prefix
     **in either direction** — a `db` watcher reacts to a `db.pool` event and vice versa;
   - `gs.RefreshProperties()` — the process-level refresh facade mounted by the
     app (no injected refresher field).
4. `App.RefreshProperties()` (`app.go:247-255`): re-loads **all** configured sources
   (files, env, cmd args), re-merges by priority, and pushes the new storage into the
   container via `c.RefreshProperties(p)` — which re-resolves every `gs.Dync[T]` binding
   atomically (`gs_core/injecting`). Non-Dync `value:` fields are **not** re-bound.
5. Success is logged per instance:
   `config bus: refreshed properties on event (prefix="..." origin="...")`.
   Failure of the refresh itself logs `config bus: property refresh failed: ...` and the
   signal is consumed — no retry, and the publisher cannot see subscriber outcomes (NATS
   core is fire-and-forget).

Note the refresh is whole-app, not scoped: `Prefix` filters *which instances react*, not
*which keys re-resolve*. A `Publish("db")` on an instance honoring it still refreshes every
`gs.Dync` field in that instance.

---

## 3. Per-key behavior reference

All keys live under the top-level absolute prefix `spring.config.bus` (bound via value tags
in `config.go`; the `nats-instance` tag sits on the bean struct itself in `bus.go`). None are
required — but see consequences.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.config.bus.subject` | string | `spring.config.refresh` | NATS subject for publish+subscribe; all instances sharing it form one bus. Read once at wiring — changing it via a refresh event does not resubscribe (§2.2). | Two fleets silently split if one instance typos the subject: refreshes propagate only within each half; no error anywhere. |
| `spring.config.bus.watch-prefixes` | string | `` (empty) | Comma-separated prefix filter, parsed once in `subscribe()`. Empty = react to every event. Non-empty = react only to empty-prefix events or bidirectional prefix overlaps (`db` ↔ `db.pool`). ⚠ dead key for hot-reload: reparsed only on restart. | Over-scoped list refreshes more than intended (harmless but noisy); a prefix that never matches published prefixes makes the instance silently skip scoped refreshes while still honoring `Publish("")`. |
| `spring.config.bus.nats-instance` | string | `config-bus` | Name of the `spring.nats.<name>.*` connection injected as transport (autowire by instance name, `bus.go:55`). ⚠ the named instance must exist — this is the de-facto activation gate. | No matching `spring.nats.<name>.*` block → container wiring failure at startup (the bean cannot be assembled). Sharing the name with a business NATS connection couples bus failures to that connection. |

Beyond these, the referenced NATS instance carries its own `spring.nats.<name>.*` keys
(url, auth, ...) — see starter-nats documentation. A NATS connection is a hard prerequisite;
there is no embedded/in-memory fallback.

---

## 4. Verification & fault drills

All drills use the §1 project; start NATS first (`docker compose up -d`).

### 4.1 Happy path: publish a prefix change, watch a gs.Dync update

```bash
docker compose up -d
GS_DEMO_MESSAGE=manual-1 go run . -manual &   # example's manual mode
# in another terminal, trigger a scoped publish (add a tiny /refresh handler in
# your app that calls d.Bus.Publish("demo") — or reuse the self-test mode):
go run .                                       # prints "config-bus refresh observed: v-..."
```

Observability: log tag `_app_config_bus` (registered via
`log.RegisterAppTag("config_bus", "")`, `starter.go:41`; tune verbosity with
`logger.config_bus.*`). Expect, in order: `subscribed to bus subject=... prefixes=[...]`,
then on each event `config bus: refreshed properties on event (prefix="..." origin="...")`.

### 4.2 Drill: publish with no subscribers

`Publish` to a subject nobody subscribes to (e.g. run one instance with a different
`spring.config.bus.subject`): the call returns `nil` — NATS core delivery is fire-and-forget.
Lesson: the publisher has **no confirmation** that anyone refreshed; corroborate via
subscriber logs or application behavior. There is no echo of your own publish in your logs
unless your own subscription honors it.

### 4.3 Drill: wrong / non-matching prefix

1. Start with `spring.config.bus.watch-prefixes=db,cache`.
2. Publish `db` → refresh happens (`db` is watched).
3. Publish `demo` → **no refresh** — `shouldRefresh` returns false and the message is
   dropped with a debug-level line `config bus: ignoring refresh event outside watched
   prefixes (...)` (`bus.go:82-87`). This is the expected quiet opt-out; enable debug
   logging if you need to see it.
4. Publish `""` → always refreshes, regardless of the watch list.

### 4.4 Drill: malformed payload

Publish raw bytes to the subject directly:

```bash
docker run --rm --network host nats:2.10 nats -s nats://127.0.0.1:4222 pub spring.config.refresh 'not-json{'
```

Subscriber logs `ignoring malformed refresh event: ...` (Warn) and stays healthy. An
**empty** payload is not malformed — it is a valid full-fleet refresh (`bus.go:74-81`). JSON
with unknown fields is tolerated (standard unmarshal).

### 4.5 Drill: startup trap reproduction

- **Missing NATS instance**: comment out `spring.nats.config-bus.url` and run — startup
  fails during container wiring on the unresolvable `configBus` bean. There is no
  `enabled=false` escape hatch; remove the import instead.
- **Self-hosted key**: add `spring.config.bus.subject=other.subject` to a source that only
  arrives via refresh (e.g. set only in the env used for a later publish) — the bean stays
  on the boot-time subject. Boot-time binding wins by construction (§2.2).

### 4.6 Refresh failure observability

If a source fails to re-load (bad file, invalid content), `RefreshProperties` returns an
error and the subscriber logs `config bus: property refresh failed: ...`; previous property
values remain in effect (the refresh is all-or-nothing per `app.go` doc comment). The signal
is not retried — republish after fixing the source.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup fails wiring `configBus` | No NATS instance named per `nats-instance` (default `config-bus`) under `spring.nats.*` | Define `spring.nats.config-bus.url=...`, or set `nats-instance` to an existing instance name |
| Publish succeeds, no instance refreshes | Subject mismatch (fleet split) or every subscriber's `watch-prefixes` excludes the published prefix | Align `subject` across instances; check overlap direction (`db` matches `db.pool`, not `demo`) |
| Refresh logged on others, not this instance | This instance joined late — NATS core has no replay; signals sent before `subscribe()` are gone | Republish after all instances are up; the bus is not a durable log |
| `property refresh failed: app not started yet` | Event arrived during the wiring window before `app.started` (§2.1) | Benign at boot; republish once started if the change mattered |
| `property refresh failed: <source error>` | A config source failed to reload; previous values kept | Fix the source, publish again — no retry is performed |
| Field didn't change after refresh | The binding is a plain `value:` field, not `gs.Dync[T]` — only Dync re-resolves | Convert the field to `gs.Dync[T]` |
| Changed `spring.config.bus.subject`/`watch-prefixes` but behavior unchanged | These keys are boot-time only, parsed once in `subscribe()` | Restart the instance |
| Changed the key a Dync reads, but value stale | The new value lives in a source the app doesn't load, or is outranked by priority (e.g. file beats env) | Check the refresh logs for which sources reload; ensure the change is visible in a loaded source |
| No metrics / health for the bus | There are none — only `_app_config_bus` logs | Watch logs; a dropped subscription is otherwise silent (see §6) |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 3 |
| Required | 0 explicit, 1 implicit (the referenced NATS instance definition) |
| Quickstart external deps | 1 (NATS broker) |
| "Watch out" entries | 5 |

Design suspects (audit ledger):

- **No activation condition** (carried over from previous audit): fixed — the bean now
  registers under an `OnProperty("spring.config.bus")` gate (like the governance sources), so
  the import is inert until configured.
- **No delivery confirmation**: NATS core fire-and-forget means the publisher cannot learn
  whether any instance refreshed — consider a reply/ack pattern or an observable counter
  before trusting the bus for coordinated rollouts.
- **Boot-time-only self keys**: `subject`/`watch-prefixes`/`nats-instance` silently ignore
  hot reload; a warn on detecting a self-key change during refresh would surface misuse.
- **No observability beyond logs**: no health indicator, no metrics (events received /
  refreshes done / failures) — a dead subscription is silent.
- **Silent scoped opt-out**: fixed — a filtered event now logs a debug-level line
  (bus.go:82-87), making "why didn't this instance refresh" diagnosable without a restart.
- **Refresh granularity**: `Prefix` filters instances, not keys — every honoring instance
  refreshes its whole property set. If that proves expensive, a key-scoped refresh path may
  be needed.
