# starter-gin Observability Smoke Test

Demonstrates and verifies OpenTelemetry observability for starter-gin:
trace context propagation, metrics emission, and trace-log correlation.

## What This Test Verifies

1. **Trace propagation**: A parent span created by the test client is propagated
   through the gin tracing middleware via W3C `traceparent`. The server-side
   span joins the same trace.
2. **Metrics emission**: The gin metrics middleware records
   `http.server.request.duration` and `http.server.active_requests` (per the
   OTel HTTP semantic conventions), exposed via Prometheus on the actuator's
   `/metrics` endpoint (`:9090`).
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
curl -v http://localhost:8001/echo/world
# The response JSON includes the inbound traceparent and log-correlated trace_id/span_id.

# (2) Scrape Prometheus metrics
curl http://localhost:9090/metrics | grep http_server_
# You should see:
#   http_server_request_duration_seconds_bucket{http_request_method="GET",http_route="/echo/:name",http_response_status_code="200",url_scheme="http",network_protocol_version="1.1",...}
#   http_server_request_duration_seconds_count{http_request_method="GET",...} 20
#   http_server_active_requests{http_request_method="GET",url_scheme="http",network_protocol_version="1.1"} 0

# (3) Check actuator health
curl http://localhost:9090/health
# -> {"status":"UP"}
```

Press Ctrl+C in Terminal 1 to stop the server.

## Architecture

```text
 [Test Process]
  |-- otel.Tracer("example-otel").Start(...)  // parent span with trace_id=abc123
  |-- Inject traceparent into HTTP header
  |-- HTTP GET http://localhost:8001/echo/world
  |       |
  |    [starter-gin observe middleware]
  |       |-- Extract traceparent from header
  |       |-- Start "GET /echo/:name" span (child of abc123)
  |       |-- Set http.request.method, url.scheme, server.address, http.route, ...
  |       |-- [handler emits log with trace_id=abc123, returns traceparent+traceID in body]
  |       |-- End span with http.response.status_code=200
  |
  |-- /metrics scrape confirms http_server_request_duration_seconds_count incremented
  |-- JSON response confirms traceparent propagated and log correlation active
```
