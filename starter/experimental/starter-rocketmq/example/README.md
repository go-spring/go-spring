# starter-rocketmq example

A minimal publish/consume round trip through the broker-neutral
`messaging.Driver`: the app subscribes to the `hello` topic in group
`hello-group`, publishes a message with a key and a custom header, and asserts
the consumer receives the same body and header. It self-exits (`SIGTERM`) once
the round trip succeeds; any failure exits non-zero.

## Run

```bash
docker compose up -d   # name server :9876 + broker :10911 (auto-creates topics)
go run .
./check.sh             # bring up, run, tear down — the smoke test
```

Manual exploration mode keeps the server running:

```bash
go run . -manual
```
