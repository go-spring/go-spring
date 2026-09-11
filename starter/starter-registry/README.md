# starter-registry

The registration core: the ONE server that owns this process's publication
lifecycle across every configured registry center.

## What it does

Each registry backend starter (`starter-registry-etcd`, `starter-registry-consul`,
`starter-registry-nacos`, `starter-registry-zookeeper`) derives one
`discovery.Registrar` per configured `${spring.registry.<backend>.<name>}`
block. This starter collects them ALL — across backends — and drives them in
lockstep:

- registered everywhere once the application is ready;
- deregistered everywhere as shutdown begins (PreStop, before any server
  stops — the lossless-drain sequence);
- weight changes broadcast to all centers (via `UpdateWeight`).

Any registration failure fails startup: a center missing this instance would
split consumers' views.

You never import this package directly — every backend starter imports it
transitively, so the single `registryServer` bean exists exactly once per
process no matter how many backends you mix.

## Configuration

```properties
# Registry centers: one NAMED block per center, any number of blocks,
# any mix of backends. Each block is a bean "<backend>.<name>".
spring.registry.etcd.main.endpoints=10.0.0.1:2379
spring.registry.etcd.dr.endpoints=10.9.0.1:2379      # dual registration
spring.registry.zookeeper.bz.servers=10.1.0.1:2181   # mixed backends

# The instance identity — ONE set of fields shared by every center.
# service-name is the registration intent signal: set = publish this process,
# unset = pure consumer (registers nothing anywhere).
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
# spring.registry.id=            # optional, derived when empty
# spring.registry.weight=100
# spring.registry.metadata.zone=cn-north

# Consumers cite a center by its bean name:
# spring.http-client.backends.users.discovery=etcd.main
```

| key | meaning |
|-----|---------|
| `spring.registry.service-name` | registration intent + the name consumers resolve |
| `spring.registry.addr` | advertised host:port |
| `spring.registry.id` | instance id within the service (derived when empty) |
| `spring.registry.weight` | initial load-balancing weight (default 100; 0 = drained) |
| `spring.registry.metadata.*` | backend-agnostic attributes |

## Runtime API

`UpdateWeight(ctx, weight)` on the `registryServer` bean re-advertises the
instance with a new weight in every center — the entry never leaves discovery;
watchers (loadbalance pools included) observe the new value on their next
snapshot.

## Notes

- The server opens no port; it plugs into the Go-Spring server lifecycle.
- `spring.registry.service-name` set with no registry center configured fails
  startup (fail-fast, not silent).
- Discovery needs no configuration of its own — see each backend starter's
  README.
