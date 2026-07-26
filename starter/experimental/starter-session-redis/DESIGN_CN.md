# starter-session-redis 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-session-redis` 属于 **Contributor** 形态(见
[starter/DESIGN.md](../DESIGN.md) §2.3),为 `spring/session`(Spring Session
的 Go-Spring 等价物)提供基于 Redis 的 `session.SessionStore`。不开端口;
贡献一个 `SessionStore` bean,由 `spring/session.Manager` 用于跨副本加载
与保存会话。

## 1. 职责与边界

- **在范围内:**把 `spring.session.redis.<name>` 一项转成一个具名
  `session.SessionStore` bean,底层复用应用已有的 `*redis.Client`。
- **不在范围内:**HTTP `Manager` 中间件与 cookie 处理(那是
  `spring/session` 的事);redis 客户端本身(那是 `starter-go-redis`)。

## 2. 关键抽象与缝隙

`spring/session` 分三段拆分,让 starter 无需触碰 HTTP 就能接入:

- **`Session`**——id、属性、`isNew` / `modified` / `invalid` / `renew`
  状态位,供中间件写回时判定。
- **`SessionStore`**——`Load` / `Save(ttl)` / `Delete`。远端后端实现更窄的
  **`ByteStore`**,经 `FromByteStore` 抬升(JSON 编码
  `sessionData{Attributes, CreatedAt}`)——与 cache 栈同款缝隙。
- **`Manager`**——唯一 HTTP 缝隙。中间件解析 cookie、加载 session、注入
  context,并在首个 header 前写回。

本 starter 贡献 `ByteStore` 实现,交由 `FromByteStore` 完成编码——形状对齐
`starter-lock-redis`。

## 3. 关键决策

- **走 bean,而非 driver 注册表。**Session 需要应用已装配的**活** redis
  客户端;把活连接注册进包级 registry 对多 run / 多 test 场景是错的。
  `spring/session` 的 driver 注册表只服务静态默认(`"memory"`);Redis 走
  `gs.Group` + `TagArg(client)`。
- **多实例经 `gs.Group("${spring.session.redis}", ...)`。**每项按名注入
  redis bean。`client` 为空时构造 fail-fast;不做 `localhost` 静默兜底
  (家族规则 §2.2)。
- **`Store` 内嵌 `session.SessionStore`。**`gs.Provide` 不能返回未导出实现;
  内嵌接口既暴露具体类型,又继承 `FromByteStore` 的所有方法。
- **TTL 就是 Redis key TTL。**过期与滑动续期由 Redis 廉价强制,无需另开
  清扫协程。`redis.Nil` 映射为 miss。
- **不注册 destroy。**starter 不 `Close()` redis 客户端(不拥有它);由
  redis starter 的 destroy 负责。

## 4. Manager 决策(上下文,住在 `spring/session`,此处镜像)

- **Cookie 恒为 HttpOnly。**去掉可配项:JavaScript 可读的 session cookie
  基本是 bug,且 `bool` 零值无法区分"显式 false"。
- **惰性分配。**从未 modified 过的新 session 不落库、不发 cookie,id 在
  首次 `Set` 才生成。
- **滑动续期。**任何携带 session 的请求都刷新 Redis key TTL 与 cookie
  `Max-Age`;空闲超过 `IdleTimeout` 才过期。
- **防会话固定。**`Session.RenewID()` 置位;commit 时删旧 id、发新 id,
  保留属性。
- **Cookie 先于 body。**`sessionWriter` 包 `ResponseWriter`,`commit()` 在
  首个 `WriteHeader` / `Write` 之前跑一次,确保 `Set-Cookie` 先出。

## 5. 取舍 / 弃选方案

- **全局 driver 注册表存活连接——弃选。**活连接进包级 map 会破坏 test /
  run 隔离。
- **按实现拆前缀——弃选。**遵循家族共享前缀规则(`spring.session`);从
  内存默认切到 Redis 后端只需换 blank-import,其它一切不动。
