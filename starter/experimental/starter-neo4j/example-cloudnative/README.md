# starter-neo4j Cloud-Native Example

A flagship for the experimental **starter-neo4j** composing the cloud-native
capability set around a graph client — **discovery**, **resilience**, **health**
and **observability**.

## Capabilities

- **Service discovery**: the neo4j address is resolved through a registered
  discovery backend (`service-name`), not hardcoded config. Because the neo4j
  driver exposes no dialer injection point, resolution happens once at startup —
  a Cypher round-trip only succeeds if resolve → dial → serve works.
- **Resilience**: with `resilience.enabled`, queries routed through
  `StarterNeo4j.Query` / `StarterNeo4j.RunWithResilience` run through the builtin
  `"default"` executor; a burst over `rate-limit` is rejected with
  `ErrRateLimited`. The resilience config is a `gs.Dync` field, so it is
  hot-reloadable.
- **Health**: the per-instance neo4j `health.Indicator` is aggregated by
  `starter-actuator` on `:9370` — `/readyz` reflects the server.
- **Dynamic config**: a `gs.Dync[string]` field is bound to a watched file;
  editing it hot-reloads the value with no restart.
- **Observability**: `StarterNeo4j.Query` emits a client span, the
  `db.client.*` metrics and an access log via the module's own instrumentation
  on the OTel globals.

## Layout

```
example.go               the flagship app (container + self-test)
conf/app.properties      discovery, resilience, actuator config
check.sh                 docker-gated smoke test
```

## Manual Testing

Requires a local Neo4j (`docker compose up -d`):

```bash
cd starter/experimental/starter-neo4j/example-cloudnative
docker compose up -d
go run . -manual
```

In another terminal:

```bash
curl http://127.0.0.1:9370/readyz                 # health (neo4j server)
```

## Smoke Test

```bash
./check.sh
```

`check.sh` brings up Neo4j via docker compose, then asserts health probes are UP,
a discovery-resolved Cypher round-trip succeeds, and a burst is partly rejected
with `ErrRateLimited`; exit code 0 means pass. Skipped gracefully when docker is
unavailable.
