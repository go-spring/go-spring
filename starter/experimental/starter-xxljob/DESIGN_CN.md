# starter-xxljob 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

手写 xxl-job 执行器 starter：以纯 HTTP 说执行器一侧的 xxl-job 协议
（注册/心跳/run/kill/log），零三方依赖。

## 1. 职责与边界

- **负责**：回调 HTTP 服务（gs.Server）承载 /run /beat /idleBeat /kill
  /log、handler 注册表、任务执行（goroutine + 可取消 context）、admin
  注册/心跳循环、任务日志文件。
- **不负责**：任务调度（admin 的事）、admin UI/数据库、任务参数解析
  （handler 拿原始串）。

## 2. 关键抽象与 Seam

- **Executor 实现 gs.Server** — 回调服务是长期运行、优雅停止的服务，不
  是 gs.Runner（starter-asynq 刚踩过的 seam，见其 DESIGN）。
- **任务 goroutine + 可取消 context** — /run 在 goroutine 上启动 handler
  并按 jobId 登记 cancel（logId 只透传给 /api/callback 与 /log）；/kill 取消它。panic 经共享 `goutil.SafeRun`
  链恢复，转成 500 回调。
- **注册循环** — `register()` 在 Executor.Run 生命周期里跑：按间隔向每个
  admin 的 /api/registry POST，返回的 closer 在关机时注销。

## 3. 约束

- 端口必填（`expr:"$ > 0"`）：admin 必须能拨回执行器，回调端口是运维的
  显式决策（见 starter-server-port-must-be-configured 规约）。
- 协议是刻意子集：registry/heartbeat/run/idleBeat/kill/log + callback。
  广播/分片（broadcastIndex/broadcastTotal）只解析不执行；GLUE（shell/
  python）执行不在范围。

## 4. 权衡 / 已否决的方案

- **手写协议 vs xxl-job-executor-go（v1.2.0，2023-05，已停更）**：社区
  SDK 陈旧且薄；线上格式就 5 个 JSON 结构 + 6 个端点，自持成本低于引入
  停更依赖。与 starter-webhook 同立场（纯 stdlib，协议即 API）。
- **mock-admin example vs docker 化 xxl-job-admin**：官方 admin 要 MySQL
  schema + JVM；被测的是执行器一侧协议，自包含 mock admin 即可覆盖（同
  starter-webhook 的自包含 example）。
