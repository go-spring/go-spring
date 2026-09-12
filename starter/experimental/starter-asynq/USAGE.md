# starter-asynq Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `client.go`, `driver.go`, `config.go`,
`health/health.go`) and the runnable [example/](example/) (docker-gated smoke: `example/check.sh`).
**Asynq's own semantics — task types, retries, scheduling, queues, the Inspector — are
[asynq's documentation](https://github.com/hibiken/asynq)**; everything below is go-spring's
increment (assembly, governance, observability, health).

**Activation**: any `spring.asynq.instances.<name>` subtree (`gs.OnProperty("spring.asynq")` is a prefix
check, starter.go:36). One instance always yields a producer `*Client`; a worker `*Server`
exists only when `<name>.server.enabled=true` (default off — a long-running worker is an
opt-in). Multi-instance: each `spring.asynq.instances.<name>` entry is an independent Redis-backed queue.

---

## 1. Complete worked project

One instance playing both roles: the producer enqueues a `example:greet` task, the worker
(enabled in config) runs it and the process self-terminates on the asserted round trip —
exactly the shape of [example/example.go](example/example.go). File tree:

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── docker-compose.yml          # redis:7
```

**go.mod** (module deps that matter — see `example/go.mod`):

```
module demo

require (
    github.com/hibiken/asynq    latest
    go-spring.org/spring        v1.3.x
    go-spring.org/starter-asynq latest
    go-spring.org/starter-actuator latest   // optional: health endpoint for §4
    go-spring.org/starter-governance latest // optional: resilience/fault on enqueue
)
```

**main.go**:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/hibiken/asynq"
    "go-spring.org/log"
    "go-spring.org/spring/gs"

    starter "go-spring.org/starter-asynq"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
)

const taskType = "example:greet"

// Service holds the producer and worker beans. The autowire tags are the
// instance name and the derived server bean name ("<name>:server").
type Service struct {
    Client *starter.Client `autowire:"a"`
    Server *starter.Server `autowire:"a:server"`
}

// completed carries the handler's payload back to the smoke assertion.
var completed = make(chan string, 1)

// Init runs after gs field-injects both beans — the correct seam to register
// handlers before the worker starts consuming (the gs Rooter Init phase runs
// before Runners/servers, see example/example.go:49-55).
func (s *Service) Init() error {
    s.Server.RegisterHandler(taskType, handleGreet)
    return nil
}

func handleGreet(ctx context.Context, task *asynq.Task) error {
    var payload map[string]string
    if err := json.Unmarshal(task.Payload(), &payload); err != nil {
        return err // a returned error enters asynq's retry domain
    }
    completed <- payload["msg"]
    return nil
}

func main() {
    svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]()).Init((*Service).Init)

    go func() {
        time.Sleep(700 * time.Millisecond)
        ctx := context.Background()
        payload, _ := json.Marshal(map[string]string{"msg": "hello asynq"})
        // Use the wrapper's Enqueue — the guarded, observed path (§2.3).
        if _, err := svrBean.Interface().(*Service).Client.Enqueue(ctx, asynq.NewTask(taskType, payload), asynq.Queue("default")); err != nil {
            log.Errorf(ctx, log.TagAppDef, "ENQUEUE failed: %v", err)
        }
        fmt.Println("Asynq round trip OK:", <-completed)
    }()
    gs.Run()
}
```

**conf/app.properties** — the complete, commented surface used above:

```properties
# --- asynq instance "a" (producer + worker share these Redis settings) -------
spring.asynq.instances.a.addr=127.0.0.1:6379
spring.asynq.instances.a.db=0
spring.asynq.instances.a.concurrency=4            # worker: max concurrent tasks
# spring.asynq.instances.a.queues=critical:5,default:1   # queue name -> priority weight
# Turn on the worker role for this instance (OFF by default).
spring.asynq.instances.a.server.enabled=true
# spring.asynq.instances.a.shutdown-timeout=8s    # worker drain bound on shutdown

# --- TLS to Redis (shared tlsconf block; off here) ---------------------------
# spring.asynq.instances.a.tls.enabled=true
# spring.asynq.instances.a.tls.cert-file=...      # + key-file / ca-file / server-name /
#                                       #   insecure-skip-verify

# --- actuator (health endpoint for §4) ----------------------------------------
spring.actuator.addr=:9370
```

**docker-compose.yml** (copied from `example/docker-compose.yml`):

```yaml
services:
  redis:
    image: redis:7
    ports:
      - "127.0.0.1:6379:6379"
```

**Verify**:

```bash
docker compose up -d
go run .                          # expect: "Asynq round trip OK: hello asynq"
./example/check.sh                # full smoke: compose up, run, assert, tear down
curl -s :9370/healthz | grep asynq   # "asynq:a": UP once Redis answers
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle timeline

```
import starter-asynq
  └─ init: gs.Module(gs.OnProperty("spring.asynq"))               [starter.go:36]
        └─ conf.BindEach("${spring.asynq}", per <name>):
             ├─ expr validation on Config (addr != '' — fail at BIND time)
             ├─ Provide(newClient).Name(<name>).Init/Destroy       [always]
             ├─ if c.Server.Enabled:
             │    Provide(newServer).Name(<name> ":server").
             │      Export(gs.As[gs.Server]()).Init/Destroy        [worker opt-in]
             └─ Provide(health.Indicator).Name("asynq:"+<name>).
                Export(gs.As[health.Indicator]())
gs.Run()
  ├─ bean wiring: app beans autowire *Client / *Server by instance name
  ├─ Rooter Init phase: app Service.Init registers handlers (mux created lazily,
  │  client.go:129-147 — registration may also happen later, but handlers are
  │  fixed once the worker starts consuming)
  ├─ Client.Init (client.go): newObserver() (observe.go),
  │    resilience.ResourceLabel("asynq", addr), fault executor armed
  ├─ Server.Init (client.go:111-127): builds asynq.NewServer(connOpt, Config{...})
  ├─ Runner phase: Server.Run — srv.Start(mux), sig.TriggerAndWait() → ready,
  │    then blocks on <-ctx.Done()
  └─ on SIGTERM: Server.Stop → srv.Shutdown() drains in-flight tasks bounded by
       shutdown-timeout; Destroy also calls Stop; Client.Destroy closes producer
       and the resilience executor
```

### 2.2 Design rationale cited from source

- **Worker is opt-in** (`server.enabled` default false): "a long-running worker is an opt-in...
  most processes only enqueue" (starter.go:33-35; starter/DESIGN.md convention).
- **`Run` uses `Start` + wait-on-ctx, never `asynq.Server.Run`**: asynq's helper installs its
  own signal handler which "would race gs's graceful-shutdown signal handling" (client.go:154-158).
  Signal handling stays in gs.
- **Handler errors/panics are asynq's domain**: the server deliberately installs no
  ErrorHandler/recover wrapper — "errors and panics inside a handler are asynq's to recover
  and retry... we keep our own reporting out of the hot path" (client.go:121-125).
- **Health probes via a fresh Inspector per check**, so it "verifies reachability without
  coupling to the producer/worker lifecycle" (health/health.go:26-32).

### 2.3 One task, layer by layer (enqueue → run)

1. App calls the **wrapper's** `Client.Enqueue(ctx, task, opts...)` (client.go:71-94). Do not
   call the promoted `*asynq.Client.Enqueue` — only the wrapper routes through the guard.
2. The observe layer opens a producer observation (`o.obs.start(ctx, "enqueue", task.Type())`,
   observe.go): span, metrics, and access log.
3. The executor runs: `fault.WrapExecutor(resilience.ExecutorFor("asynq", "asynq:<addr>"))` — with
   starter-governance, a rate-limit rejection or open circuit aborts **before** Redis is
   touched; without it the executor is a pass-through.
4. `Client.EnqueueContext` writes the task to Redis (asynq semantics: queue/priority from opts).
5. The worker's `ServeMux` matches the task type against the registered pattern (`:` groups
   for middleware scoping) and invokes your `HandlerFunc` on one of `concurrency` slots.
6. Handler returns nil → done, span ends without error. Handler returns error → **asynq's
   retry domain** (default 25 retries with exponential backoff — see
   [asynq retries](https://github.com/hibiken/asynq#retries)); the starter adds nothing.
7. A panic inside the handler is recovered by asynq's own guard and also retried.

---

## 3. Per-key behavior reference

Instance prefix: `spring.asynq.instances.<name>.*` (Config is bound via `conf.BindEach` with the prefix —
these ARE instance-prefixed).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.asynq.instances.<n>.addr` | string | — | Redis `host:port`; also feeds the governance resource label `asynq:<addr>`. Required (`expr:"$ != ''"`). | Missing → bind-time startup error. |
| `..username` / `..password` | string | empty | Redis ACL auth. | Wrong → enqueue/handler failures at runtime, not at boot (health probe catches it). |
| `..db` | int | 0 | Redis database index. | Mismatched db between producer/worker instances → tasks enqueued but never consumed. |
| `..tls.*` | block | off | Shared tlsconf (`enabled`, `cert-file`, `key-file`, `ca-file`, `server-name`, `insecure-skip-verify`); when on, DefaultDriver builds a TLS RedisClientOpt (driver.go:58-77). | Half-configured TLS → driver build error at bean construction. |
| `..concurrency` | int | 10 | Worker: max tasks processed concurrently. No effect on a producer-only instance. | Too low → queue backlog; silently irrelevant when `server.enabled=false`. |
| `..queues` | map[string]int | empty → asynq "default":1 | queue → priority weight (higher = processed more often). ⚠ your `asynq.Queue(...)` enqueue option must name a configured queue (or the fallback default), or the worker never picks it up. | Enqueue to an unlisted queue → task sits pending forever. |
| `..shutdown-timeout` | duration | 8s | Bounds the worker's drain (`srv.Shutdown()`); ctx passed to `Stop` is unused — the drain rides this timeout (client.go:176-183). | Too low → in-flight tasks abandoned mid-run on deploy. |
| `..server.enabled` | bool | false | **Worker opt-in switch.** Also gates whether the `*Server` bean exists at all — an `autowire:"a:server"` without it fails wiring. | Injecting the worker without this key → container "bean not found". |
| `..driver` | string | empty | Names the Driver bean to assemble this instance through. Empty = inject the single Driver bean by type (none → bundled DefaultDriver); naming a missing bean fails startup. | Several Driver beans coexist → select one per instance by name. |

---

## 4. Verification & fault drills

### 4.1 Round trip (job executes)

```bash
docker compose up -d && go run .        # "Asynq round trip OK: hello asynq"
./example/check.sh                      # scripted: compose -p gs-asynq-example, marker assert
```

### 4.2 Health

The per-instance indicator `asynq:a` does an Inspector round trip on `default`:

```bash
docker stop asynq-redis
curl -s :9370/healthz | grep asynq      # flips DOWN; recovers on restart
docker start asynq-redis
```

### 4.3 Retry on failure (asynq domain)

Make the handler return an error once (e.g. fail on first delivery):

```go
if atomic.AddInt32(&tries, 1) == 1 { return fmt.Errorf("boom") }
```

The task re-runs per asynq's retry policy (default 25, exponential backoff) — watch:

```bash
redis-cli -n 1 keys '*'                  # asynq keeps retry state in Redis
grep -c "boom" <log>                     # handler error surfaces via asynq's logs
```

### 4.4 Drain on shutdown

Start a slow handler (sleep 3s), send a task, then `kill -TERM` the process during the run:
the worker drains up to `shutdown-timeout` (8s default) before exiting; a timeout shorter
than the run abandons it (asynq then retries it on the next delivery — asynq semantics).

### 4.5 Governance guard (optional)

With starter-governance + a `govern` source, open the circuit / set a rate limit on resource
`asynq:<addr>`: `Client.Enqueue` returns the rejection **without touching Redis**; the
promoted `asynq.Client` path would bypass the guard entirely (see §5 row 2).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Container fails: bean `a:server` not found | `server.enabled` not set (worker opt-in) | Set `spring.asynq.instances.<n>.server.enabled=true` or drop the autowire. |
| Tasks enqueued but never run | Worker not enabled; enqueue to a queue not in `queues`; producer and worker on different `db` | Align config; enqueue with `asynq.Queue("<listed>")`. |
| Guard/resilience never applies | Calling promoted `*asynq.Client.Enqueue/EnqueueContext` instead of the wrapper | Call the wrapper's `Enqueue` (client.go:71). |
| Health DOWN though enqueue works | `default` queue never created / ACL limits Inspector | Health checks the `default` queue specifically; ensure Redis reachable and permissions. |
| Tasks lost on deploy | `shutdown-timeout` shorter than in-flight run | Raise it above the longest expected task. |
| Custom Driver bean never used | The entry did not select it and by-type injection picked another | When several `Driver` beans coexist, select one per instance: `spring.asynq.instances.<name>.driver = <bean-name>` (empty = the single Driver bean by type; naming a missing bean fails startup). |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 10 instance-prefixed (incl. 6 tls sub-keys) |
| Required | 1 (`addr`) |
| Quickstart external deps | 1 (Redis) |
| "Watch out" entries | 5 |

Design suspects (kept from the prior edition, plus new findings; the former
"dead driver selection" entry is fixed):

- Producer and worker share one Config although only addr/auth/tls are truly common;
  `concurrency`/`queues`/`shutdown-timeout` are worker-only keys at top level.
- The promoted `*asynq.Client` methods (`EnqueueContext`, etc.) bypass the guard/observation
  seam — easy to call by accident.
