# starter-actuator 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-actuator` 属于 Server 形态（`starter/DESIGN.md` §2.1）：在独立端口
上跑一个管理 HTTP server，暴露 K8s 探针、build info、运行时自省——是业务
server 的运维对偶物。

## 1. 职责与边界

- 服务 `/healthz`、`/readyz`、`/startupz`（K8s liveness / readiness /
  startup 探针；旧名 `/health`、`/readiness`、`/startup` 作为别名保留），
  以及 `/info`、`/loggers`、`/env`、`/configprops`、`/threaddump`。
- 采集所有导出为 `health.Indicator` 的 bean，把状态汇聚到 `/readyz`；
  actuator 不认识具体后端（redis / gorm……）——缝隙是 stdlib 接口。
- 采集所有 `endpoint.Endpoint` bean 并挂到该 server 上，让一个管理端口
  同时承担 actuator + otel `/metrics` + 未来贡献者，跨 starter 无 import。
- 与 app 主 HTTP server（通过不同 bean `Name`）以及 `starter-pprof`、
  `starter-admin-ui` 并存——各自一个端口。

## 2. 关键抽象与缝隙

- **在启动期就服务，而非等 ready。** server bind 后立即 Serve；`TriggerAndWait`
  仅用来观察整体聚合。这是刻意的：readiness 探针必须能观察到
  OUT_OF_SERVICE→UP 的翻转，liveness 探针必须在漫长启动期间持续应答否则
  pod 会被过早杀掉。
- **`health.Indicator` 在 stdlib。** 接口放 `spring/health` 而非 starter——
  容器只按显式 `.Export(As[Iface]())` 建索引，且该接口必须对每个贡献者
  （redis / gorm……）都可达而无需 import 本 starter。
- **`PreStop` 翻转 readiness。** `PreStop` 置 `draining=true`，让 `/readyz`
  返回 503 OUT_OF_SERVICE，同时其他 server 继续处理 in-flight 请求；K8s
  endpoint controller 摘走该 pod，drain 延时到期后再关 server。
- **Endpoint 贡献。** 内置 pattern 先注册；贡献 endpoint（例如 otel
  `/metrics`）后挂。pattern 重复会让 `ServeMux` 在启动 panic——错配识别面。
- **`/readyz` 每次 sweep 有超时。** 一轮 readiness 扫描被 `checkTimeout`
  钳制，防止单个慢 indicator 拖垮探针超出 K8s 超时。

## 3. 约束

- **默认端口 `:9370`。** 与 app 主 HTTP server（`:9090`）、pprof
  （`127.0.0.1:9981`）区分；绑所有 interface 让集群内探针可达。
- **JSON body 限制。** POST `/loggers/{name}` 解码上限 64 KiB 且
  `DisallowUnknownFields`——防止畸形/超大 POST 耗尽内存或静默吞错。
- **Indicators / Endpoints / Env 都是可选（`autowire:"?"`）。** 应用无
  indicator 仍能拿到 liveness/readiness/info。
- **分组语义。** liveness 忽略非 liveness 组的 indicator；readiness 与 startup
  各自要求本组通过——降级依赖不该重启 pod，只该切流量。

## 4. 权衡 / 已否决方案

- **复用 app 主 HTTP mux——否决。** 探针必须在启动期与 drain 期都能应答，
  所以需要一个生命周期与 app readiness gate 解耦的 listener。
- **让各 starter 把 indicator 推过来（而非 actuator 拉）——否决。** 推模型
  强迫每个后端 starter import actuator；靠接口导出拉，让 actuator 有没有
  后端都能工作。
