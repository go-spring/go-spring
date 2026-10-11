# starter-governance-nacos Example

Nacos-backed governance rule source with starter-governance-nacos.

## Features

- **Cold load**: The rules document is read from its own dataId at startup and seeds the governance policy
- **Rule hot push**: Publish a new document to the dataId; the listener delivers it and the policy flips in real time — no restart, and no app-wide property re-bind
- **Governance isolation**: The rule document is governance's own; enabling it never turns on app-property refresh

> Requires a Nacos on `127.0.0.1:8848` — `docker-compose.yml` brings one up (the smoke test does this for you).

## Manual Testing

```bash
cd starter-governance-nacos/example
docker compose up -d
go run . -manual
```

The app keeps running and prints the resolved policy every second:

```
policy: enabled=true timeout=100ms retries=2
```

In another terminal, publish a new document and watch it flip (900ms):

```bash
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
  --data-urlencode 'dataId=gs-govern-demo.yaml' \
  --data-urlencode 'group=DEFAULT_GROUP' \
  --data-urlencode 'content=spring:
  governance:
    enabled: true
    client:
      default:
        enabled: true
        attempt-timeout: 900ms
        max-retries: 2'
# prints: policy: enabled=true timeout=900ms retries=2
```

Press `Ctrl+C` to stop; `docker compose down -v` tears the server down.

## Smoke Test

```bash
./check.sh
```

`check.sh` brings up Nacos via docker compose, runs the example (no `-manual`), which
seeds a 100ms document, cold-asserts the policy, publishes a 900ms document and
verifies the push — exit code 0 means pass.
