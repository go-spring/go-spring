# starter-registry

The registration family core: the ONE server that owns this process's
publication lifecycle across every configured registry center.

**Applications never import this module.** Every backend starter
(`starter-registry-etcd`, `-consul`, `-nacos`, `-zookeeper`) depends on it, so
it comes along transitively and the single `registryServer` bean exists exactly
once per process no matter how many backends you mix.

## What it does

Each backend starter derives one `discovery.Registrar` per configured
`${spring.registry.<backend>.<name>}` block. This core collects them ALL —
across backends — and drives them in lockstep:

- registered everywhere once the application is ready;
- deregistered everywhere as shutdown begins (PreStop, before any server
  stops — the lossless-drain sequence);
- weight changes broadcast to all centers (via `UpdateWeight`).

Any registration failure fails startup: a center missing this instance would
split consumers' views.

## Configuration

The registry configuration surface splits into **three parts**, each independent
and each optionally present on its own:

| part | answers | keys | who sets it |
|------|---------|------|-------------|
| ① instance identity | who am I | `${spring.registry}.*` | providers only |
| ② registry centers | where to register / discover | `${spring.registry.<backend>.<name>}.*` | providers and/or consumers |
| ③ citation | which center a consumer uses | each client starter's own config | consumers only |

### ① Instance identity — `${spring.registry}.*`

ONE set of fields shared by every center. `service-name` is the registration
intent signal: **set = publish this process; unset = pure consumer** (registers
nothing anywhere).

```properties
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080      # required when registering
# spring.registry.id=                   # optional, derived when empty
# spring.registry.weight=100            # 0 = drained; negative normalized to 1
# spring.registry.version=              # app version, consumers may canary on it
# spring.registry.zone=                 # availability zone, consumers may prefer same-zone
# spring.registry.scheme=               # transport hint (tcp/tls/http/https)
# spring.registry.metadata.zone=cn-north
```

| key | meaning |
|-----|---------|
| `spring.registry.service-name` | registration intent + the name consumers resolve |
| `spring.registry.addr` | advertised host:port |
| `spring.registry.id` | instance id within the service (derived when empty) |
| `spring.registry.weight` | initial load-balancing weight (default 100; 0 = drained) |
| `spring.registry.version` | application version of the instance (optional) |
| `spring.registry.zone` | availability zone of the instance (optional) |
| `spring.registry.scheme` | transport hint, mapped to consumers' `Endpoint.Scheme` (optional) |
| `spring.registry.metadata.*` | backend-agnostic attributes |

### ② Registry centers — one NAMED block per center

Any number of blocks, any mix of backends (etcd + zookeeper in one process is
fine). Each block is a bean named `<backend>.<name>` serving **both halves**:
the write side collected by this core as a Registrar, and the read side cited
by consumers as a Discovery backend. Each backend's full key set lives in its
starter's README/USAGE.

```properties
spring.registry.etcd.main.endpoints=10.0.0.1:2379
spring.registry.etcd.dr.endpoints=10.9.0.1:2379      # dual registration
spring.registry.zookeeper.bz.servers=10.1.0.1:2181   # mixed backends
```

### ③ Citation — consumers name a center by its bean name

Written in each client starter's own config; discovery itself needs no
configuration, and the registry never knows who cites it.

```properties
# http-client:
spring.http-client.instances.users.service-name=users
spring.http-client.instances.users.discovery=etcd.main    # bean name "<backend>.<name>"

# gateway:
spring.gateway.discovery=nacos.main                       # process-wide default
# spring.gateway.routes.<id>.upstream.discovery=...       # per-route override

# redigo (redis):
spring.redigo.instances.cache.service-name=cache-svc
spring.redigo.instances.cache.discovery=consul.main

# gorm (databases, per dialect):
spring.gorm.mysql.orders.service-name=mysql-svc
spring.gorm.mysql.orders.discovery=etcd.main

# mongodb:
spring.mongodb.instances.logs.service-name=mongo-svc
spring.mongodb.instances.logs.discovery=nacos.main

# discovery supports a family-level default: an instance block that sets none
# falls back to ${<family>.default.discovery}.
spring.http-client.default.discovery=etcd.main           # family default
spring.http-client.instances.pay.service-name=pay-svc
spring.http-client.instances.pay.discovery=etcd.main     # instance override (omitted = default)
```

Every client starter that routes through discovery with a `loadbalance.Pool`
follows this shape: `service-name` + `discovery=<bean name>` on the instance's
own config. Each starter's full key set lives in its README/USAGE.

### How the three parts relate

- **① and ② are decoupled**: a block plus `service-name` registers, even if no
  consumer ever cites it; conversely a pure consumer configures blocks with no
  `service-name` and registers nothing.
- **`service-name` set with no block = startup fails** (fail-fast; see Notes).
- **k8s appears only in ② and ③**: it is a discovery-only backend — citable,
  never registered through (the platform already registers Pods), so it never
  touches ①.
- **One process can be both provider and consumer**: configure ① + ② + ③.

## Runtime API

`UpdateWeight(ctx, weight)` on the `registryServer` bean re-advertises the
instance with a new weight in every center — the entry never leaves discovery;
watchers (loadbalance pools included) observe the new value on their next
snapshot.

## Notes

- The server opens no port; it plugs into the Go-Spring server lifecycle.
- This core ships no registry backend of its own. With
  `spring.registry.service-name` set and no center configured, startup fails
  fast ("no registry center is configured") rather than registering nowhere —
  add a `starter-registry-<backend>`.
- Discovery needs no configuration of its own — see each backend starter's
  README.
