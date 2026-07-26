# starter-admin-ui 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-admin-ui` 属于 Server 形态（`starter/DESIGN.md` §2.1）：一个轻量、
自包含的 dashboard，按固定周期轮询一组应用实例的 `starter-actuator` 端点
并渲染聚合状态表。

## 1. 职责与边界

- 默认在 `:9280` 跑独立 `http.Server`；与 actuator（`:9370`）、pprof
  （`:9981`）和 app 主 server 分开。
- 对配置里的每个实例按固定周期轮询 `/health` / `/readiness` / `/startup`
  / `/info`，聚合以 HTML 和 `GET /api/status` JSON 呈现。
- 定位有意窄。Prometheus + Grafana 是推荐的聚合监控方案；本 starter 面向
  该栈缺席的情况（本地/on-prem、临时 bring-up、局部检查）。定位为
  "用 Go 惯用法达等价效果"，而非 Spring Boot Admin 的功能级复刻。

## 2. 关键抽象与缝隙

- **零三方依赖。** 一个 HTML 模板以 Go 字符串常量内嵌、一个轮询 goroutine、
  `net/http`。air-gap 环境原样能跑，不需要 CDN，不需要外部资源。
- **读优化的 snapshot。** 轮询 goroutine 是唯一写方，页面 handler 复制
  snapshot 后返回，从不阻塞在一次实时 poll——过期 snapshot 好过慢页面。
- **每次 sweep 上限 = interval。** 每次刷新在被 `context` 钳到 interval，
  防止拥塞实例让相邻 sweep 交叠。
- **启动期就服务。** 与 actuator 一致，不阻塞在 `sig.TriggerAndWait`——
  dashboard 在启动期就可达，运维能观察聚合状态翻转。
- **部分失败仍可渲染。** `/info` 失败静默忽略（nice-to-have）；只有
  `/health` 不可达才写入行 `Error`。`/readiness` 非 2xx（503 带 body）也
  会解码，让状态 pill 正确渲染。

## 3. 约束

- **`Instances` 是静态列表。** 不接 discovery——目标集合是
  `spring.admin-ui.instances`（逗号分隔的 actuator base URL）；此 UI 面向
  已知的小规模实例集，不是全集群扫描器。
- **默认端口 `:9280`。** 可通过 `spring.admin-ui.addr` 覆盖。
- **auto-refresh 至少 1s。** 亚秒 poll interval 会被钳到 HTML `meta
  refresh` 下限。
- **行按 base URL 排序。** 保证表格在刷新之间不抖动。

## 4. 权衡 / 已否决方案

- **完全复刻 Spring Boot Admin——否决。** SBA 有自注册、JMX、通知器、
  历史存档——K8s 栈里要么被 Prometheus + Grafana 顶上，要么只在大 Java
  fleet 里才划算。
- **从 discovery 后端取实例列表——v1 否决。** 目标场景是 bring-up / on-prem，
  discovery 后端本身可能是被验证对象。后续可以在同一渲染上加 discovery 模式。
- **SSE / WebSocket 推送——否决。** HTML `meta refresh` 10s 足够撑状态表，
  也让整个 starter 保持在几百行 Go。
