# starter-governance-etcd Example

Etcd-backed governance rule source with starter-governance-etcd.

## Features

- **Cold load**: The rules document is read from its own etcd key at startup and seeds the governance policy
- **Rule hot push**: Publish a new document to the key; the watch stream delivers it and the policy flips in real time — no restart, and no app-wide property re-bind
- **Governance isolation**: The rule document is governance's own; enabling it never turns on app-property refresh

> Requires an etcd on `127.0.0.1:2379` — `docker-compose.yml` brings one up (the smoke test does this for you).

## Manual Testing

```bash
cd starter-governance-etcd/example
docker compose up -d
go run . -manual
```

The app keeps running and prints the resolved policy every second:

```
policy: enabled=true timeout=100ms retries=2
```

In another terminal, publish a new document and watch it flip (900ms):

```bash
docker exec starter-governance-etcd etcdctl put /gs-govern-demo.yaml 'spring:
  governance:
    enabled: true
    client:
      enabled: true
      resilience:
        timeout: 900ms
      max-retries: 2'
# prints: policy: enabled=true timeout=900ms retries=2
```

Press `Ctrl+C` to stop; `docker compose down -v` tears the cluster down.

## Smoke Test

```bash
./check.sh
```

`check.sh` brings up etcd via docker compose, runs the example (no `-manual`), which
seeds a 100ms document, cold-asserts the policy, publishes a 900ms document and
verifies the push — exit code 0 means pass.
