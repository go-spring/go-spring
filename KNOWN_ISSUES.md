# 未结案缺陷与待办

本文件是**维护者面向**的清单,只收**未解决的问题**:未定位的缺陷、仍开放/明确不修的框架项、以及受阻的待办。纯事实(机制说明、版本参数、既有设计决策)不入此文件——它们是规约或常识,归各自的模块文档。

落档日期:2026-10-06。**里面的判断都带日期,取用前先对着当前代码核一遍**——同期已发现多批记忆内容与代码脱节。

---

## 一、未结案的缺陷

### 1.1 `starter-http-client` 开 resilience 后对直连后端 100% 失败(未定位)

发现于 2026-08-12,给 http-client 接 fault(放火)时端到端验证撞见,**根因至今未定位**。

**现象**:开 resilience 后对直连后端 100% 请求失败、熔断器秒开;关 resilience(plain transport)则 0 错误。

**复现**:`starter-http-client/example-load`,conf 设 `spring.http-client.load.resilience.enabled=true`(任意 error-threshold / max-retries / timeout)+ 内嵌后端 `127.0.0.1:18080`(curl 直连正常返回 200)。

**当时的判断**:不是 fault 接线引入的(fault off 时路径等价)。怀疑是 resilience 的 RoundTripper 与 httpx 声明式 transport(`fixedHostTransport` → `otelhttp` → `DefaultTransport`)交互的问题——疑似把成功 200 误计为失败,或重试导致后端拒连,从而熔断秒开。影响的是 http-client 的 resilience 整体可用性,不止 fault。

**排查方向**:对比包与不包 httpx transport 时 breaker 的 success/failure 计数(RoundTripper 的返回值 + 5xx 判定)。

**⚠ 现状待重新确认(2026-10-06)**:当年的 `cloud/resilience/roundtripper.go` **已不存在**,机制演进为 `cloud/resilience/client_adapter_http.go` 的 `NewRoundTripper(base, exec)`(现在 influxdb、gateway 在用)。**这条缺陷是否仍在,只能真跑 `example-load` 才能判定**——本轮整理时未跑。

---

## 二、未完成的框架项(2026-09-06 扩展审计遗留)

来自 2026-09-06「扩展特化」全仓审计的**仍 open 低优先项**。都**不阻断扩展**,属遗留打磨。**2026-10-06 复核:仍成立**,但部分文件路径已迁移(见各条注)。

- **F-5 `starter-scheduler` 缺 bean 缝**:`Server.Run` 里无条件 `scheduling.NewScheduler()`,私有无 `OnMissingBean`(同族 lock 用 `Lockers map[string]lock.Locker autowire:"?"` 真 bean 缝)。已判分布式/编排调度排除(转 `starter-xxljob`)。**触发条件 = 第二个进程内后端落地**才加 `Scheduler autowire:"?"`——勿为单一实现预开抽象。
  - *注(2026-10-06):路径已从 `starter/experimental/starter-scheduler` 变为 `starter/starter-scheduler`;`NewScheduler()` 仍无条件调用,条目仍成立。*
- **`starter-grpc` 的扩展缝是包级 registry**:`RegisterBalancer` 等走包级注册 + RWMutex(init 期注册、wiring 期读),进程级共享、无法逐 server 组合(对照 redigo 折叠单 wrap / gin 的 bean 形)。experimental 阶段可接受,**转正前宜改 bean 缝**。
  - *注(2026-10-06):原记录写的 `UseUnaryInterceptor` **在代码里不存在**(全仓零命中)。grpc 实际导出的是拦截器构造器(`AccessLogUnaryInterceptor` / `LoadTest...` / `Fault...` / `Recover...` / `Metrics...` / `Tracing...`,各有 Unary/Stream)加 `RegisterBalancer` 这一族。*

---

## 三、其它已知失败与受阻项

### 结构性坑与端口

- gs 内置 `SimpleHttpServer` **默认开启**且 addr 默认 `:9090`,与 starter-otel prometheus exporter 的 `spring.observability.metrics.port=9090` 必然撞端口(`bind: address already in use`)。不需要 HTTP 的 example 加 `spring.http.server.enabled=false`;两者都要就错开 `spring.http.server.addr` 或 `metrics.port`。`metrics.port=0` 把 `/metrics` 交给 starter-actuator 托管,未引 actuator 会打 WARN(2026-09-13)。
- `spring.governance.driver` 认 conf 文件不认 `app.Property`;治理 `ExecutorFor` 首次调用若早于 GoLive(provider 注册)会落 noop(懒解析),测试要等就绪。
- `starter-goframe` 及 contrib/goframe 系 4 个模块编译失败是上游 `otelmetric v2.10.2` × `contrib/instrumentation/runtime` 的版本冲突(`Version` 由 func 变 const),**已知且明确不修**,全仓 `go build` 直接忽略;2026-08-14 起 12 个 goframe 模块已从 go.work `use` 移除,单独 build 用 `GOWORK=off`(2026-08-13)。

### 待补跑

- `starter-milvus` 的 docker 冒烟**未跑通**:`milvusdb/milvus:v2.4.2` 在当前网络不可得(Docker Hub 超时、daocloud 403、1ms.run not found、xuanyuan 429)。代码 / 构建 / 单测全绿,`check.sh` 已就绪——网络恢复后补跑。
