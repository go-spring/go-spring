# starter-otel 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-otel` 属于 Global/infrastructure 形态（`starter/DESIGN.md` §2.4）
的 starter：根据 `${spring.observability}` 构建进程级 OpenTelemetry
`TracerProvider` / `MeterProvider` 并装成 OTel 全局，让所有被埋点的组件
（starter-gorm-*、http/gRPC middleware……）无需逐组件接线
即可打通。

## 1. 职责与边界

- 拥有共享的 trace + metrics provider、propagator、resource 属性。不安装
  log-trace 关联 hook：`log.FieldsFromContext` 缝隙是进程级单点，覆盖面太窄，
  不宜由 starter 绑死一种安装器，故交由应用自行掌握（见示例）。
- 接线模型是**隐式全局**，不是逐 bean 注入：导入 starter 即是启用；每个
  OTel-aware 库都读全局。
- provider 不是 bean。它们是模块 setup 阶段装入的进程全局，经
  `gs.RegisterStopper` 关停——该时机在所有 server 停止、容器关闭之后。
- 可选地把 Prometheus `/metrics` handler 贡献为 `endpoint.Endpoint`，让
  `starter-actuator`（如有）在共享管理端口上挂载，跨 starter 无 import。

## 2. 关键抽象与缝隙

- **`gs.Module` 里 eager 建 provider。** 模块体在 `applyModules`
  （RefreshPrepare）阶段运行，即所有 bean 构造函数之前。这里调
  `otel.SetTracerProvider` / `SetMeterProvider` 保证下游
  `db.Use` / `otelhttp.NewHandler` 都能看到真实 provider；把构建放到 bean
  构造函数会打破顺序。
- **Endpoint 贡献缝隙。** 使用 pull（Prometheus）exporter 且
  `metrics.port=0` 时，把抓取 handler 以 `endpoint.Endpoint` bean 暴露；
  `starter-actuator` 收集所有 `endpoint.Endpoint` 并挂在 `:9370`——运维
  只需一个抓取端口。`metrics.port>0` 时改用独立 `http.Server`。
- **`log.FieldsFromContext` 缝隙（starter 不安装）。** 该缝隙是进程级单点
  hook，在每条日志记录里写入当前 span 的 `trace_id`/`span_id`。缝隙有意只
  允许一个 installer，而应用可能想要的关联策略范围太宽，starter 不替它选，
  因此把缝隙交给应用。示例自行安装了标准 hook。
- **runtime metrics 作为可选增值。** `runtime` 埋点挂在同一个 MeterProvider
  上，`mp.Shutdown` 一并回收——不用单独的 stop hook。
- **Enable=false 完全空操作。** 保留 SDK 空 provider；导入但未启用的 starter
  对运行时无副作用。

## 3. 约束

- **每进程一个 TracerProvider / MeterProvider。** 全局单例；第二个
  `starter-otel` 式安装器会打架。
- **exporter=`none` 关闭该柱。** 与 `enable: false` 效果一致但支柱级别粒度
  （比如 trace 开 metrics 关）。
- **Prometheus exporter 是 pull 型。** 二选一：port=0（挂 actuator）或
  port>0（独立 server），不能都用。
- **关停顺序：server → 容器 → OTel flush → 日志。** provider 注册为进程级
  stopper（`gs.RegisterStopper`），运行时只在所有 server 停止、容器关闭后才
  调用——此刻不再有任何新 span/metric 产生。flush 的 ctx 无 deadline
  （`context.WithoutCancel`），最后一批数据会等导出完成，不会被关停超时掐断。
- **硬杀进程丢什么。** `Shutdown` 是唯一的 flush 点。进程没跑到 stopper 就
  死掉（SIGKILL、`os.Exit`、server 优雅退出永不返回），缓冲数据即丢：
  trace 最多一个 BatchSpanProcessor 队列（默认 2048 条 / 5 s 批超时），
  metric 最多一个 PeriodicReader 采集周期（默认 60 s）。pull 型 Prometheus
  exporter 无缓冲——只在被抓取时现算，不存在积压可丢。

## 4. 权衡 / 已否决方案

- **逐组件注入 `trace.Tracer` bean——否决。** 让每个组件依赖 tracer bean
  会强制跨 starter import，且违背 OTel 全局模型。生态标准 API 的意义就是
  库读全局。
- **把 exporter/支柱切成各自 starter——否决。** 拆开会重复 resource 组装，
  运维还要同步一堆 enable 开关。一个 starter，一个 config 树。
