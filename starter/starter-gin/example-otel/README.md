# starter-gin Observability Smoke Test

Demonstrates and verifies OpenTelemetry observability for starter-gin:
trace context propagation, metrics emission, and trace-log correlation.

## What This Test Verifies

1. **Trace propagation**: A parent span created by the test client is propagated
   through the gin tracing middleware via W3C `traceparent`. The server-side
   span joins the same trace.
2. **Metrics emission**: The gin metrics middleware records
   `http.server.request_count`, `http.server.request_duration`, and
   `http.server.active_requests`, exposed via Prometheus on the actuator's
   `/metrics` endpoint (`:9370`).
3. **Log correlation**: Log lines emitted during request handling carry
   `trace_id` and `span_id` from the active span (via starter-otel's
   `log.FieldsFromContext` hook).

## Quick Smoke Test

```bash
cd starter/starter-gin/example-otel
./check.sh
```

Exit code 0 means all observability assertions passed.

## Manual Verification

Terminal 1 — start the server in manual mode:
```bash
cd starter-gin/example-otel
go run . -manual
```

Terminal 2 — send a traced request:
```bash
# (1) Send a request and inspect the response headers
curl -v http://localhost:8001/echo-trace/world
# The response JSON includes the inbound traceparent and log-correlated trace_id/span_id.

# (2) Scrape Prometheus metrics
curl http://localhost:9370/metrics | grep http_server_
# You should see:
#   http_server_request_count_total{http_method="GET",http_route="/echo-trace/:name",...}
#   http_server_request_duration_seconds_bucket{http_method="GET",http_route="/echo-trace/:name",...}
#   http_server_active_requests{http_method="GET",http_route="/echo-trace/:name"}

# (3) Check actuator health
curl http://localhost:9370/health
# -> {"status":"UP"}
```

Press Ctrl+C in Terminal 1 to stop the server.

## Architecture

```text
 [Test Process]
  |-- otel.Tracer("example-otel").Start(...)  // parent span with trace_id=abc123
  |-- Inject traceparent into HTTP header
  |-- HTTP GET http://localhost:8001/echo-trace/world
  |       |
  |    [starter-gin tracingMiddleware]
  |       |-- Extract traceparent from header
  |       |-- Start "HTTP GET" span (child of abc123)
  |       |-- Set http.method, http.target, http.scheme, net.host.name
  |       |-- [handler emits log with trace_id=abc123, returns traceparent+traceID in body]
  |       |-- End span with http.status_code=200
  |
  |-- /metrics scrape confirms http_server_request_count_total incremented
  |-- JSON response confirms traceparent propagated and log correlation active
```
