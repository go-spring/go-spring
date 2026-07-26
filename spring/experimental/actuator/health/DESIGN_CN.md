# health 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`health` 位于 stdlib(零依赖基础层),让任何 starter 都能实现它、任何收集方
都能消费它,而两侧都不必互相 import。

## 1. 职责与边界

- **做:** 定义 `Indicator` 契约,命名与 Kubernetes 对齐的三个探针分组,提供
  可选细化 `Grouped`,以及小助手(`GroupsOf` / `InGroup`)方便收集方按探针
  过滤。
- **不做:**
  - 不提供 HTTP / gRPC / RPC 表面,不做聚合逻辑,不做检查调度。这归收集方。
  - 不依赖 DI 容器。`Indicator` 是普通 Go 接口;接线是收集方的事。

## 2. 关键抽象与缝隙

- **`Indicator` 有意保留两方法。** 更窄(只是 `func(context.Context) error`)
  就丢了聚合输出所需的稳定组件名;更宽会把调度/上报塞进契约。
- **分组镜像 K8s 容器生命周期。** liveness / readiness / startup 直接映射到
  容器探针,同名让收集方省一层翻译。
- **`Grouped` 可选,默认安全。** 未实现 `Grouped` 的 indicator,`GroupsOf`
  返回 `{readiness, startup}`,**绝不**含 `liveness`。让依赖检查影响 liveness
  会因下游抖动重启 Pod,得不偿失。
- **收集方通过 `Export` `Indicator` 自动装配。** Go-Spring 容器只按具体类型
  与显式 `Export` 的接口做类型索引(见 `spring/gs`)。把 `Indicator` 放在
  stdlib 里,让贡献方(如 `starter-go-redis`)与收集方
  (`starter-actuator`)都只依赖 stdlib —— 正是 `starter/DESIGN.md` §3 拆分的
  意图。
- **bean 懒实例化。** 贡献的 indicator bean 只有被收集方装配时才会构建。没有
  收集方,贡献零开销。

## 3. 不变量

- `HealthName()` 短、稳、应用内唯一(聚合输出的 map key)。
- `CheckHealth` 必须遵守 `ctx`(超时、取消);慢依赖不能拖住探针。
- Indicator 并发安全。
- 任何 indicator 都不默认进 `GroupLiveness`;必须 opt-in。

## 4. 权衡与放弃的方案

- **接口放 stdlib 而非收集方 starter。** 放 `starter-actuator` 会让每个贡献方
  都 import actuator —— `starter/DESIGN.md` 禁止的跨 starter 依赖。stdlib 是
  两侧都能依赖的唯一基础。
- **两方法接口,返回值不用状态枚举。** 返 `error` 更 Go 惯例,让聚合器自行构造
  更丰富的判定;`StatusUp` / `StatusDown` 只保留字符串给聚合报告用。
- **不做异步 / 缓存健康。** 具体策略(限速探测、回退窗口)由收集方拿捏,它
  才知道多久探一次、对外怎么暴露。把它们塞进契约会锁死策略。
