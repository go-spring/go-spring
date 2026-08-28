# starter-xxljob Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `executor.go`, `protocol.go`,
`registry.go`) and the self-contained runnable [example/](example/) (smoke: `example/check.sh`,
no docker). **Job semantics — scheduling, routing/block strategies, the admin console — are
[xxl-job's documentation](https://www.xxl-job.com)**. The executor protocol is hand-rolled here
(no third-party Go SDK) and covers the admin callbacks `/run`, `/beat`, `/idleBeat`, `/kill`,
`/log`; everything below is go-spring's increment plus the exact protocol surface we implement.

**Activation**: any `spring.xxljob.<name>` subtree. Multi-instance: one executor per entry.
Three keys are required at bind time (`app-name`, `admin-addresses`, `port` — `expr` validation
in config.go:30-41), so a half-configured instance fails at startup, not at first trigger.

---

## 1. Complete worked project

The example embeds a mock admin (just enough REST to register and trigger) so the whole
trigger→run→callback path runs locally — exactly [example/example.go](example/example.go).
Production points `admin-addresses` at a real xxl-job-admin. File tree:

```
demo/
├── go.mod
├── main.go            # executor + handler registration + self-test
└── conf/
    └── app.properties
```

**go.mod** (see `example/go.mod`):

```
module demo

require (
    go-spring.org/spring        v1.3.x
    go-spring.org/starter-xxljob latest
)
```

**main.go** (abridged from the example — mock-admin scaffolding omitted; see the file):

```go
package main

import (
    "context"
    "fmt"
    "net/http"

    "go-spring.org/spring/gs"

    starter "go-spring.org/starter-xxljob"
)

type Service struct {
    Executor *starter.Executor `autowire:"a"`
}

// Init registers handlers after the executor bean is injected and before the
// callback server starts serving (executor.go:61-63: names are fixed once the
// server is up).
func (s *Service) Init() error {
    s.Executor.RegisterHandler("demoJob", func(ctx context.Context, param string) error {
        // nil => success reported to the admin; non-nil => failure + message.
        // Write task logs to <log-dir>/<logId>.log yourself if the admin
        // should see them via /log (the starter only SERVES that dir).
        return nil
    })
    return nil
}

func main() {
    gs.Provide(&Service{}).Export(gs.As[gs.Rooter]()).Init((*Service).Init)
    gs.Run()
}
```

**conf/app.properties** — the complete, commented surface (copied from
`example/conf/app.properties`):

```properties
# --- xxl-job executor instance "a" -------------------------------------------
spring.xxljob.a.app-name=go-spring-demo          # registered with the admin
# Production: http://<admin-host>:8080/xxl-job-admin (list = load-balanced).
spring.xxljob.a.admin-addresses=http://127.0.0.1:18081
# Callback server port — the admin must be able to dial back; an explicit
# operator decision (no default port).
spring.xxljob.a.port=9999
# Re-register/heartbeat period (10s default; example tightens it for the smoke).
spring.xxljob.a.registry-interval=5s
# Per-task log dir served back via /log (files are yours to write).
spring.xxljob.a.log-dir=./logs
# Access token, only when the admin has one set (sends XXL-JOB-ACCESS-TOKEN).
# spring.xxljob.a.access-token=...
```

**Verify** (mirror of `example/check.sh`):

```bash
go run .                          # expect: "xxl-job round trip OK: demoJob ran"
./example/check.sh                # scripted marker assert
# Drive the callback server directly in manual mode (go run . -manual):
curl -s -XPOST :9999/beat                          # {"code":200,"msg":""}
curl -s -XPOST -d '{"jobId":1,"executorHandler":"demoJob","executorParams":"a=1","logId":1}' :9999/run
curl -s -XPOST -H 'Content-Type: application/json' -d '{"jobId":1}' :9999/idleBeat  # 200 idle / 500 "job running"
curl -s ":9999/log?logId=1&fromLineNum=0"          # LogResult JSON
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle timeline

```
import starter-xxljob
  └─ init: gs.Module(gs.OnProperty("spring.xxljob"))              [starter.go:30]
        └─ conf.BindEach("${spring.xxljob}", per <name>):
             ├─ expr validation on Config (app-name != '', admin-addresses
             │  non-empty, port > 0 — BIND-time failure)
             ├─ Provide(newExecutor).Name(<name>).
             │   Export(gs.As[gs.Server]()).Destroy(...)           [starter.go:32-35]
             └─ Provide(health.Indicator).Name("xxljob:"+<name>).
                Export(gs.As[health.Indicator]())                  [starter.go:37-39]
gs.Run()
  ├─ bean wiring: app autowires *Executor by instance name
  ├─ Rooter Init phase: app Service.Init calls RegisterHandler
  ├─ newExecutor (construction, executor.go:72-90): mux with
  │   /run /beat /idleBeat /kill /log; http.Server on :port
  │   (ReadHeaderTimeout 5s)
  ├─ Runner phase: Executor.Run (executor.go:94-112)
  │   ├─ prepare(): outboundIP() via UDP dial 8.8.8.8 (registry.go:100-108),
  │   │   ensureLogDir() mkdir -p
  │   ├─ register(ctx): ticker loop — registryOnce() POSTs
  │   │   {"registryGroup":"EXECUTOR","registryKey":app-name,
  │   │    "registryValue":"http://<ip>:<port>/"} to every admin's
  │   │   /api/registry; stop fn POSTs /api/registry/remove
  │   ├─ sig.TriggerAndWait() → ready
  │   └─ ListenAndServe until ctx.Done → srv.Shutdown
  └─ on SIGTERM: StopContext → http.Server.Shutdown(ctx) (drains callbacks);
       deferred stopRegistry removes the registration on the way out
```

### 2.2 Design rationale cited from source

- **Port is required, no default**: "an executor server must be reachable by the admin to
  receive trigger callbacks, so the port is an explicit operator decision" (config.go:37-41).
- **Protocol hand-rolled, no SDK**: "It speaks the xxl-job executor protocol to an admin ...
  over plain HTTP, hand-rolled (no third-party SDK) — see DESIGN" (config.go:25-27).
- **Tasks run on their own goroutine with cancellable ctx**: "`/kill` can interrupt a long
  task; a panic in a task is recovered through the shared goutil panic chain" (executor.go:17-21;
  the run path wraps the fn in `goutil.SafeRun`, executor.go:167-169).
- **/run returns immediately**: "runs a task in a new goroutine and returns immediately; the
  task posts its completion back to the admin via /api/callback" (executor.go:142-143) —
  xxl-job's async trigger model.

### 2.3 One trigger, layer by layer

1. The admin's scheduler fires job `1` → picks this executor from the registry and POSTs
   `/run` with a `TriggerParam` (`jobId`, `executorHandler`, `executorParams`, `logId`, ...)
   (protocol.go:27-40).
2. `handleRun` (executor.go:144-178) decodes the body, looks up `registry[executorHandler]`;
   unknown handler → `{"code":500,"msg":"no handler registered for ..."}` and nothing runs.
3. A cancellable ctx is stored under `running[jobId]`, and the task starts on a fresh goroutine.
4. `/run` replies `{"code":200}` immediately (async model); the admin's timeout governs from
   its side (`executorTimeout` in the trigger param is the admin's concern).
5. The task body runs under `goutil.SafeRun`: an error becomes `{code:500, msg:err}`; a panic
   is recovered by the shared panic chain and reported as failure.
6. On completion the executor POSTs the official `HandleCallbackParam` JSON array
   (`[{logId, logDateTime, handleCode, handleMsg}]`) to **every** `admin-addresses` entry's
   `/api/callback` (executor.go), with `XXL-JOB-ACCESS-TOKEN` when configured; the callback
   context survives a kill so the admin still gets the killed job's outcome.
7. Meanwhile `/beat` answers liveness, `/idleBeat` (JSON body `{"jobId":N}`) answers whether
   that job is running (block-strategy input), `/kill` (JSON body `{"jobId":N}`) cancels the
   running task(s) of that job, `/log?logId=&fromLineNum=` serves the log slice from
   `log-dir/<logId>.log`.

---

## 3. Per-key behavior reference

Prefix: `spring.xxljob.<name>.*` (multi-instance).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `app-name` | string | — | Registry key the admin groups executors under. Required (`expr:"$ != ''"`). | Bind-time startup error. |
| `admin-addresses` | []string | — | Admin base URLs; registry/heartbeat and task callbacks fan out to **every** entry. Required (`len > 0`). | Bind-time startup error; a wrong-but-present URL registers nowhere and only shows as silent no-registration. |
| `port` | int | — | Callback server port; must be reachable from the admin. Required (`> 0`). | Bind-time startup error; port conflict surfaces at `ListenAndServe` as a Run error. |
| `access-token` | string | empty | Sent as `XXL-JOB-ACCESS-TOKEN` on executor→admin calls when non-empty. ⚠ must match the admin's token exactly; empty matches only a token-less admin. | Mismatch → admin silently drops registry/callback requests. |
| `registry-interval` | duration | 10s | Re-register/heartbeat period. ⚠ must be well under the admin's registry-expiry (default 90s in stock xxl-job) or the executor flaps. | Too large → executor periodically offline from the admin's view; triggers route elsewhere. |
| `log-dir` | string | `./logs` | Dir served by `/log` as `<LogId>.log`; the starter creates it if absent. ⚠ division of labor: the starter owns the **reading** side only — your TaskFunc owns writing `<log-dir>/<logId>.log`. | Empty logs in the admin console. |

---

## 4. Verification & fault drills

### 4.1 Trigger round trip (job executes)

```bash
go run .                                        # "xxl-job round trip OK: demoJob ran"
./example/check.sh                              # scripted marker assert
```

### 4.2 Probing the callback endpoints (manual mode)

```bash
go run . -manual &
curl -s -XPOST :9999/beat ; echo
curl -s -XPOST -H 'Content-Type: application/json' \
     -d '{"jobId":1,"executorHandler":"demoJob","executorParams":"a=1","logId":1001}' :9999/run ; echo
curl -s -XPOST -H 'Content-Type: application/json' -d '{"jobId":1}' :9999/idleBeat ; echo  # 500 "job running" while the task runs
curl -s -XPOST -H 'Content-Type: application/json' -d '{"jobId":1}' :9999/kill ; echo      # 200 cancels; 500 "job not running" otherwise
curl -s ":9999/log?logId=1001&fromLineNum=0" ; echo
```

### 4.3 Kill drill (interrupt a long task)

Register a handler that `select { case <-ctx.Done(): ... case <-time.After(30*time.Second): }`,
trigger it, then POST `{"jobId":1}` to `/kill` — the task observes ctx cancellation and the
completion callback reports the interruption. Running jobs are tracked by `jobId` (the key the
admin uses for /kill and /idleBeat); the trigger's `logId` is only carried through to the
`/api/callback` payload and `/log`. The example proves the split by triggering
`jobId=2, logId=2002` and killing by jobId (`example/example.go`, "kill round trip OK").

### 4.4 Failure reporting

Make the handler return an error: the admin-side callback carries `handleMsg:<err>` with
`handleCode:500`. Panics are recovered (goutil chain) and report as failure — the process survives.

### 4.5 Shutdown drain

`kill -TERM` during a running task: `StopContext` calls `http.Server.Shutdown(ctx)` — the
callback server drains in-flight HTTP; running task goroutines are not forcibly stopped by the
starter (no stored cancel sweep on shutdown). Registration removal is POSTed on the way out.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Container fails at startup on config | `app-name`/`admin-addresses`/`port` missing or empty | All three are expr-required at bind time (config.go:30-41). |
| Executor never receives triggers | `port` unreachable from the admin (firewall/k8s NetworkPolicy); outbound IP mis-detected | Check the admin's executor list shows `http://<ip>:<port>/`; `outboundIP` uses a UDP dial to 8.8.8.8 (registry.go:100-108) — on multi-NIC hosts it may pick the wrong interface. |
| `/run` answers 500 "no handler registered" | Handler not registered before serving, or name typo vs the admin's JobHandler | Register in a Rooter `Init` (before Runner phase); names must match exactly. |
| Executor registers then flaps offline | `registry-interval` too large vs admin expiry | Keep it well under the admin's registry-expiry (stock default 90s). |
| Admin shows empty task logs | The starter serves `log-dir` but never writes it | Write `<log-dir>/<logId>.log` from your TaskFunc (you can key it by the trigger's logId). |
| Kill/terminate job has no effect | Admin's jobId not matching the trigger's job (running table is keyed by jobId) | Verify you're POSTing `{"jobId":N}` (JSON body) with the same jobId the trigger carried. |
| Health says UP but the admin is unreachable | The indicator only checks "server built" (executor.go:253-260) | Watch registration logs; consider an admin-probe of your own. |
| Token mismatch, nothing registers | `access-token` differs from the admin's | Align both sides; empty only matches a token-less admin. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 6 |
| Required | 3 (`app-name`, `admin-addresses`, `port`) |
| Quickstart external deps | 1 (an xxl-job admin; the example mocks it) |
| "Watch out" entries | 4 |

Design suspects (kept from the prior edition, plus new findings):

- **Fixed 2026-08-28 — /run vs /kill+idleBeat keying mismatch** (was: `running` keyed by
  `LogID`, `/kill`+`/idleBeat` looked up by `jobId`, so kill could never hit a stock admin):
  the running table is now keyed by `jobId` with `logId` carried only to `/api/callback` and
  `/log`; /kill and /idleBeat decode the official JSON bodies (`KillParam`/`IdleBeatParam`).
  Covered by `executor_test.go` (TestKillByJobIDNotLogID) and the example's kill drill.
- **Fixed 2026-08-28 — /api/callback payload shape** (was: `LogResult`-shaped body, which a
  stock admin silently drops): now POSTs the official `HandleCallbackParam` JSON array
  (`logId`/`logDateTime`/`handleCode`/`handleMsg`); the callback also rides a context that
  survives a kill cancellation. Covered by TestCallbackPayloadShape and the example.
- Health indicator only checks "server built", not that registration with the admin
  succeeded — an unreachable admin surfaces only via registry logs.
- **Fixed 2026-08-28 — health indicator display name vs bean name mismatch** (was: the
  indicator was named after `app-name` while the bean name used the instance `<name>`):
  the indicator is now named `xxljob:<name>` from the instance name, matching the bean name
  and the sibling-starter convention. Covered by `TestHealthIndicatorNamedByInstance`.
- **Fixed 2026-08-28 — dead `observability.*` binding removed**: the Config's Observability
  field was bound but never read; per the repo's dead-config principle the field, schema
  entry, and docs were deleted (no observer layer was added).
- The `/api/callback` body uses the `LogResult` shape rather than the stock admin's
  `HandleCallbackParam` (`{logId, handleCode, handleMsg}`); round-tripping against a real
  admin is asserted in comments (protocol.go:17-20) but only the mock exercises it.
- **Fixed 2026-08-28 — log-dir division of labor made explicit** (was: the starter owns
  `log-dir` and the `/log` reader but never writes task logs, an undocumented split): now
  documented on the Config field and here — the starter creates and **serves** the dir via
  `/log`; the application's TaskFunc owns writing `<log-dir>/<logId>.log`. No file-writing
  was added to the starter.
