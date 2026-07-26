# cache 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`cache` 是 stdlib(零依赖基础层)的缓存抽象:定义后端实现的接口,并桥接到
`aspect`,让缓存关注点一次声明、任何后端可承接。

## 1. 职责与边界

- **做:** 定义 `Cache` 接口;提供进程内默认实现 `Memory`;提供字节型 seam
  `ByteStore` + `Codec` 让远端后端共用一条序列化路径;组合成 `MultiLevel`
  多级缓存;暴露包级驱动注册表;桥接到 `aspect.Store` 实现 `@Cacheable` 等价。
- **不做:**
  - 不提供具体远端后端。Redis / memcached / bigcache 各自在 starter 里实现
    并按名注册。
  - 不做缓存击穿保护、异步刷新、负缓存。这些策略更适合放在 aspect 层或
    调用方,而非通用接口。

## 2. 关键抽象与缝隙

- **`Cache` 值类型是 `any`。** 与 `aspect.Joinpoint.Result` 对称,命中时能直接
  短路 aspect 链,边界处不必做类型转换。
- **`ByteStore` + `Codec` 是唯一序列化 seam。** 远端后端原生只存字节,通过
  `FromByteStore` 提升为 `Cache`,所有远端共享同一条序列化路径。starter 只
  实现 `ByteStore`,不要在各处重复写编解码。
- **驱动注册表**(`Register` / `Get` / `MustGet`)与 `discovery.Register`、
  `resilience.RegisterDriver` 同构:空名、nil 后端、重复注册在 init 期 panic,
  让接线错误早失败。
- **`AsStore` 桥到 `aspect`。** 后端错误当 miss,`Set` 失败被吞:一个坏掉的
  缓存绝不能让业务调用失败,与 aspect 的 fail-open 契约一致。
- **`MultiLevel` 近读远读、全层写。** 单层读错不打断扫描(近端失败不能掩盖
  远端命中);写/删错误 `errors.Join` 汇总,方便看到部分故障。

## 3. 不变量

- 注册表要允许 package init 期填充、运行期并发读,单个 `sync.RWMutex` 足够。
- 后端必须并发安全。
- 只有 `Memory` 与 `ByteStore` 编解码两种形态。新的非字节后端要么直接实现
  `Cache`(罕见,仅另一层进程内缓存有意义),要么改走 `ByteStore`。
- `NewMultiLevel` 至少要有一层;空层级是 bug,不是可容忍的降级状态。

## 4. 权衡与放弃的方案

- **值 `any` + JSON 默认编解码。** 把"值 any"与"字节后端"之间的张力用一条
  JSON 路径收口。对具体 struct 类型是有损的(struct 出来是 `map[string]any`),
  这是有意的取舍。近端 `Memory` 不受影响(保留具体类型),多级部署下本地读
  仍是快且带类型的。
- **不做 sliding-window / 击穿保护。** 更适合由调用方(如 aspect 链上加
  single-flight,或直接用 Redis 原语)表达,不塞进通用接口。
- **不设全局默认后端。** 调用方要么直接传入 `Cache`,要么按名查表;没有隐式
  "the cache",让多租户/多域的接线保持显式。
