# starter-gin 可观测性冒烟测试

演示并验证 starter-gin 的 OpenTelemetry 可观测能力：
链路追踪传播、指标暴露、以及日志-追踪关联。

## 验证内容

1. **链路传播**: 测试客户端创建的父 Span 通过 W3C `traceparent` 头传播，
   经由 gin 追踪中间件提取后，服务端 Span 加入同一 Trace。
2. **指标暴露**: gin 指标中间件记录的 `http.server.request.duration` 和
   `http.server.active_requests`（遵循 OTel HTTP 语义约定），
   通过 actuator 的 `/metrics` 端点（`:9090`）以 Prometheus 格式暴露。
3. **日志关联**: 请求处理期间产生的日志行携带当前 Span 的
   `trace_id` 和 `span_id`（通过 starter-otel 的
   `log.FieldsFromContext` 钩子注入）。

## 快速冒烟测试

```bash
cd starter/starter-gin/example-otel
./check.sh
```

退出码为 0 表示所有可观测性断言通过。

## 手动验证

终端 1 — 启动服务（手动模式）：
```bash
cd starter-gin/example-otel
go run . -manual
```

终端 2 — 发送带追踪的请求：
```bash
# (1) 发送请求并查看响应头
curl -v http://localhost:8001/echo/world
# 响应 JSON 包含入站 traceparent 和日志关联的 trace_id/span_id

# (2) 抓取 Prometheus 指标
curl http://localhost:9090/metrics | grep http_server_
# 应看到 http_server_request_duration_seconds_bucket、http_server_request_duration_seconds_count
# 和 http_server_active_requests 等指标

# (3) 检查 actuator 健康端点
curl http://localhost:9090/health
# -> {"status":"UP"}
```

在终端 1 按 Ctrl+C 停止服务。

## 架构

```text
 [测试进程]
  |-- otel.Tracer("example-otel").Start(...)  // 父 Span, trace_id=abc123
  |-- 将 traceparent 注入 HTTP 请求头
  |-- HTTP GET http://localhost:8001/echo/world
  |       |
  |    [starter-gin observe 中间件]
  |       |-- 从请求头提取 traceparent
  |       |-- 创建 "GET /echo/:name" Span（abc123 的子 Span）
  |       |-- 设置 http.request.method, url.scheme, server.address, http.route 等属性
  |       |-- [handler 产生携带 trace_id=abc123 的日志，返回 traceparent+traceID]
  |       |-- 结束 Span，记录 http.response.status_code=200
  |
  |-- /metrics 抓取确认 http_server_request_duration_seconds_count 已递增
  |-- JSON 响应确认 traceparent 已传播且日志关联已激活
```