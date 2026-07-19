# endpoint 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`endpoint` 是位于 stdlib(零依赖基础层)的一个 seam:让一个 starter 向另一个
starter 的管理端口贡献 HTTP 路径,而两边都不必 import 对方。与
`health.Indicator` 位置相同、用法一致。

## 1. 职责与边界

- **做:** 定义一个极小的接口 —— `Path() string` + 内嵌 `http.Handler`,
  以 `Endpoint` 形式导出的 bean 实现它。
- **不做:**
  - 不做收集器、不做 mux、不做 serve 代码。这归管理端口 owner(通常是
    `starter-actuator`)。
  - 不定义路径分类或保留名。贡献方与收集方按约定用;冲突是接线 bug,由使用方
    自己解决。

## 2. 关键抽象与缝隙

- **基于接口的收集,配合容器 `Export` 契约。** Go-Spring 容器只按具体类型 +
  bean 显式 `Export` 的接口做类型索引。把接口放在 stdlib 里,让贡献方(如
  `starter-otel` 暴露 `/metrics`)与收集方(`starter-actuator`)都只依赖
  stdlib,不互相 import,遵守 `starter/DESIGN.md` §3。
- **`Path()` 是值,不是接线细节。** 贡献方拥有路径,收集方只负责按告知挂载。
  即使日后累加更多路径(`/build-info` / `/threaddump` ...),接口仍稳定。

## 3. 不变量

- 实现必须并发安全(管理端口的探针并发触达)。
- `Path()` 在 bean 生命周期内应保持静态、稳定。
- 不要与 actuator 自带路径(`/health` / `/readiness` / `/info`)冲突;运行时
  不强制,靠 code review 约束。

## 4. 权衡与放弃的方案

- **接口而非注册函数。** `RegisterEndpoint(path, handler)` 也能实现,但它反转
  了控制权(贡献方要在 init 中主动调用、还得先知道收集方存在)。bean 本就是
  Go-Spring 装配可选贡献的机制,懒实例化让未使用的 endpoint 零开销。
- **两方法接口,不用 struct。** 保留为接口(而非 `type Endpoint struct
  { Path string; Handler http.Handler }`)让贡献方以后可以在同一对象上添加
  运维方法(健康元数据、版本等)而不必拆包。
