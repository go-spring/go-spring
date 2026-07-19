# Full-stack Reference App

An end-to-end sample that proves the Go-Spring starters cooperate in one request
path: **gateway → order service (A) → inventory service (B)**, with service
discovery, a config center, a distributed transaction, observability, and
security all wired together at once.

Its real value is as an **integration harness**: assembling this many starters in
one place surfaces the conflicts that only appear when everything runs together
(port allocation, drain ordering, trace propagation gaps, config prefixes). The
findings are catalogued in [INTEGRATION_NOTES.md](INTEGRATION_NOTES.md).

> Consumes existing starters only. No new starter is introduced here.

## Topology

Three independent processes, one Go module (`fullstack`), one binary per
`cmd/` directory:

| Process   | Framework                | Business port | Actuator | Registers as | Role |
|-----------|--------------------------|---------------|----------|--------------|------|
| gateway   | starter-gateway          | `:9440`       | `:9370`  | —            | edge, routes `/api/**` → `lb://order` |
| order     | built-in HTTP mux        | `:8081`       | `:9371`  | `order`      | JWT resource server, runs the Saga |
| inventory | starter-gin              | `:8082`       | `:9372`  | `inventory`  | stock reserve/release (Saga downstream) |

Dependencies (Docker): **Consul** (`:8500`) for discovery/registration,
**Nacos** (`:8848`) for the config-center hot-reload.

## Request flow

```
client --POST /api/orders (Bearer token)--> gateway :9440
   gateway  stripPrefix(1), lb://order via Consul -----> order :8081
      order  JWT verify (starter-security-jwt)
      order  Saga step 1 "reserve"  --HTTP--> inventory :8082  /reserve
      order  Saga step 2 "charge"   (fails iff Nacos flag set)
             |__ on failure: compensate --HTTP--> inventory /release
   <-- 200 committed   (both steps ok)
   <-- 409 compensated (charge-fail=true; reservation released on B)
```

- **Discovery**: the gateway's `lb://order` and order's `order→inventory` call
  both resolve through a Consul-backed `discovery.Discovery`
  (`internal/consuldisc`), registered once under the name `consul`. Instances
  register themselves via `starter-registry-consul`.
- **Config center**: `fullstack.order.charge-fail` is imported from Nacos into a
  `gs.Dync[bool]`. Publishing it live flips the next order into the compensation
  path — no restart.
- **Distributed transaction**: `starter-transaction-saga`. Step 1 reserves stock
  (compensation releases it); step 2 charges. A charge failure rolls back the
  reservation on the *other* service.
- **Observability**: `starter-otel` + `starter-actuator`. Each service exposes
  probes and `/metrics` on one management port; logs carry `trace_id`; the trace
  propagates order→inventory (W3C headers). See the notes for the gateway gap.
- **Security**: `starter-security-jwt`. The order service is the resource server;
  the gateway forwards the caller's `Authorization` header untouched.
- **Graceful shutdown**: on SIGTERM readiness flips to `OUT_OF_SERVICE`, then the
  framework drains before stopping servers (`app.shutdown.pre-stop-delay`).

## Run it

```bash
# From this directory. Brings up Consul + Nacos, boots the three services,
# drives the full request path, and asserts committed + compensated + drain.
./check.sh
```

Or by hand:

```bash
docker-compose up -d                     # consul + nacos
go run ./cmd/inventory &                 # :8082
go run ./cmd/order &                     # :8081
go run ./cmd/gateway &                   # :9440

TOKEN=$(go run ./cmd/mint alice user)
curl -i -X POST http://127.0.0.1:9440/api/orders                       # 401
curl -i -X POST http://127.0.0.1:9440/api/orders -H "Authorization: Bearer $TOKEN"  # 200 committed

# Flip the config in Nacos, then order again -> 409 compensated:
curl -X POST http://127.0.0.1:8848/nacos/v1/cs/configs \
  --data-urlencode dataId=gs-fullstack-order \
  --data-urlencode group=DEFAULT_GROUP \
  --data-urlencode 'content=fullstack.order.charge-fail=true'
curl -i -X POST http://127.0.0.1:9440/api/orders -H "Authorization: Bearer $TOKEN"  # 409 compensated
```

## Layout

```
cmd/gateway     edge; registers Consul discovery, blank-imports starter-gateway
cmd/order       JWT resource server + Saga coordinator + Nacos-driven flag
cmd/inventory   gin service; reserve/release; registers into Consul
cmd/mint        prints an HS256 bearer token (shared secret, no external IdP)
internal/consuldisc   Consul-backed client-side discovery.Discovery
internal/authsecret   the shared HMAC secret
```

See also: [中文说明](README_CN.md) · [Integration notes](INTEGRATION_NOTES.md)
