# health 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`health` 是零依赖基础包:只 import 标准库,让任何 starter 都能实现它、任何
收集方都能消费它,而两侧都不必互相 import。

## 1. 职责与边界

- **做:** 定义 `Indicator` 契约(名 + 检查 + 探针分组 + 关键性),命名与
  Kubernetes 对齐的三个探针分组,提供 `NewIndicator` 工厂与
  `WithGroups` / `NonCritical` 选项,让 starter 贡献 indicator 时无需手写 struct。
- **不做:**
  - 不提供 HTTP / gRPC / RPC 表面,不做聚合、不调度、不定义路由策略--连
    默认分组路由也不做。`GroupsOf` / `InGroup` 在收集方(`starter-actuator`);
    本包只声明 indicator *是什么*,不管它 *怎么被消费*。
  - 不依赖 DI 容器。`Indicator` 是普通 Go 接口;接线是收集方的事。

## 2. 关键抽象与缝隙

- **`Indicator` 四方法。** `HealthName`(身份)与 `CheckHealth`(检查)是核心;
  `HealthGroups` 与 `IsCritical` 承载收集方所需的元数据(路由与严重性)。这两者
  原是可选细化接口(`Grouped`、`Critical`),折成必需方法后,`NewIndicator` 构造
  的单一类型即可满足全部契约,收集方无需 type-assert 分支。手写 `Indicator`
  只需提供平凡默认(`HealthGroups` 返 nil、`IsCritical` 返 true)。
- **分组镜像 K8s 容器生命周期。** liveness / readiness / startup 直接映射到
  容器探针,同名让收集方省一层翻译。
- **`HealthGroups` 为空表示"无意见",而非"哪都不进"。** 收集方套用默认--约定为
  readiness + startup,绝不含 liveness--因此依赖检查不会因下游抖动触发 Pod 重启。
  默认路由是收集方的契约,不在本包;把它留在外面,才守住"对结果如何被消费保持
  沉默"的边界。
- **收集方通过 `Export` `Indicator` 自动装配。** Go-Spring 容器按导出接口匹配 bean
  与注入点。把 `Indicator` 放在基础包,让贡献方(如 `starter-go-redis`)与收集方
  (`starter-actuator`)都只依赖本包。
- **bean 懒实例化。** 贡献的 indicator bean 只有被收集方装配时才构建。没有收集方,
  贡献零开销。

## 3. 不变量

- `HealthName()` 短、稳、应用内唯一(聚合输出的 map key)。
- `CheckHealth` 必须遵守 `ctx`(超时、取消);慢依赖不能拖住探针。
- Indicator 并发安全。
- 依赖检查的 `HealthGroups` 不要返回 `GroupLiveness`;liveness 只用于自检,不用于
  下游资源,且必须显式 opt-in。

## 4. 权衡与放弃的方案

- **接口放基础包而非收集方 starter。** 放 `starter-actuator` 会让每个贡献方都
  import actuator--跨 starter 依赖。基础包是两侧都能依赖的唯一共享底座。
- **必需方法,而非可选细化接口。** `Grouped`、`Critical` 作为可选接口能让裸 struct
  以更少方法满足 `Indicator`,但迫使收集方 type-assert,并把实现拆成两个类型
  (`indicator` / `groupedIndicator`)。折成必需方法,手写实现只需多一个
  `return nil` / `return true`,构造路径也收敛为单一类型。
- **返回值不用状态枚举。** 返 `error` 更 Go 惯例,让聚合器自行构造更丰富判定;
  `StatusUp` / `StatusDown` 只保留字符串给聚合报告用。
- **不做异步 / 缓存健康。** 具体策略(限速探测、回退窗口)由收集方拿捏,它才
  知道多久探一次、对外怎么暴露。把它们塞进契约会锁死策略。
