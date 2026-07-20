# aspect 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`aspect` 位于 stdlib(零依赖基础层),为 Go-Spring 提供 AOP 的等价能力,让
应用与框架自身都能挂横切逻辑而不引入框架布线。

## 1. 职责与边界

- **做:** 把有序拦截器围在目标函数或 HTTP handler 外,提供带类型的入口
  `Chain.Run` / `RunE` / `Around[T]`,并附带一组通用内置拦截器(事务、缓存、
  计时、panic 恢复、切点)。
- **不做:**
  - 不做字节码织入、不做反射动态代理、不做代码生成。Go 缺乏运行时元对象协议,
    强行复刻 Java AOP 要么牺牲类型安全,要么牺牲容器简洁。
  - 不改 `spring/`、`gs/` 与容器,不引入 `BeanPostProcessor` 等价钩子。挂横切
    靠写一个与实现共享接口的装饰器 bean。
  - 不做具体事务/缓存后端。`TxManager` 与 `Store` 是接口,真实后端(gorm、
    redis 等)在 starter 层。

## 2. 关键抽象与缝隙

- **`Chain` 边界是 `any`**,不是泛型参数。`Cache` 这类拦截器要能读/替换返回值,
  无论其具体类型;强类型链会阻断此需求。`Around[T]` 在调用点用一次类型断言把
  静态类型还原回来,全程零反射。
- **`Joinpoint.Proceed` 显式传递 context。** 拦截器派生新 context(例如
  `Transactional` 把事务句柄放进 context)后传给 `Proceed`,下游从 context 拿到
  事务,符合 Go 的传播惯例。
- **`Store` 与 `TxManager` 是可插拔缝隙。** 后端只实现这一小段接口;starter
  以 bean 形式满足(参见 `go-spring.org/spring/cache.AsStore` 桥接)。拦截器
  本身不 import 任何具体后端。
- **`NewHandler` 是 HTTP 缝隙。** 与 `resilience.NewHandler` 同构:5xx 会被
  `Timing` 等拦截器视为失败;即便配了重试策略,请求也只服务一次 —— 入站服务
  不是幂等的。

## 3. 不变量

- nil / 空链必须是透明透传(与 `resilience.Executor` 契约一致),没配拦截器
  时布线保持零开销。
- index 0 是最外层;`Chain.With` 只向内追加,不修改接收者。
- `Chain` 与所有内置拦截器必须并发安全。
- 除 Go 标准库外零依赖。新增内置拦截器如需具体后端,必须以接口形式接入,
  不能 import 具体实现。

## 4. 权衡与放弃的方案

- **边界用 `any` vs 泛型链。** 选 `any`,让异构拦截器(Cache 读结果、
  Transactional 不看结果)共享一条链;静态类型损失只在边界,由 `Around[T]`
  在调用点复原。
- **装饰器 + DI vs 容器内代理。** 选装饰器:调用方只依赖接口,分不清拿到的
  是原实现还是装饰;容器内代理要么反射包装(牺牲类型安全),要么代码生成
  (引入构建期依赖),都不采纳。
- **内置 `MemoryStore`。** 框架必须开箱可缓存,在测试里也可用;远端缓存
  (Redis、memcached)通过同一 `Store` 接口接入,不动拦截器。
