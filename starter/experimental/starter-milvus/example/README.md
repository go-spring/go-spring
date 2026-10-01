# starter-milvus example

A round trip against Milvus standalone: create a collection (dim 8), insert a
vector, load, search, and assert the top-1 hit. Self-exits on success.

## Observability

The starter only **declares** what each RPC is (`observe.go`: the span name, the
`db.client` metric prefix, the `db.system`/`db.operation` labels, and the full method path as
span/log detail). The **resilience layer emits** — the one point on the executor chain that
sees a whole call, retries included — so every call reports a call-level
`db.client.operation.duration` histogram, an attempt-level `db.client.attempt.duration`
histogram, and an access log tagged `_app_milvus_access`. With governance off it is all a
no-op; metrics additionally need `starter-otel` to install providers.

## Run

```bash
docker compose up -d   # etcd + minio + milvus standalone
go run .
./check.sh             # bring up, run, tear down — the smoke test
```
