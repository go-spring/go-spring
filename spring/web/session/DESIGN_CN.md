# session 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`session` 是 stdlib 层零依赖的服务端 HTTP 会话抽象。分布式后端在 starter
里(`starter-session-redis`);内置 `Memory` store 让 stdlib 自足可测试。

## 1. 职责与边界

- 从请求进入 manager middleware 到响应 header 写出之间,拥有 session 全生命
  周期:load / create / attach / mutate / write-back / rotate / destroy。
- 不是身份提供者。session 属性可任意存,但"调用者是谁"来自 `spring/security`。
- 不是分布式存储。`SessionStore` 是缝隙;远端 store 通过实现 `ByteStore` 贡
  献 bean。

## 2. 关键抽象与缝隙

- `Session`——状态 + 非持久标志位(`isNew`、`modified`、`invalid`、`renew`),
  Manager 在写回时消费。`snapshot()` 复制可持久部分供 store 序列化。
- `SessionStore`——`Load` / `Save(ttl)` / `Delete`。**`ByteStore` +
  `FromByteStore`** 缝隙类比 `cache.ByteStore`:starter 只需在自家 client 上
  实现 `Get/Set/Delete []byte` 的窄接口,即可通过 `sessionData` 的 JSON 编码
  抬升成完整 `SessionStore`。所有远端后端共享一条序列化路径。
- 包级 `Register` / `Get` / `MustGet`——driver-registry 范式,针对进程静态
  store。`Memory` 在 `init()` 注册为 `"memory"`。
- `Manager`——唯一 HTTP 缝隙。`Middleware` 读 cookie、加载或新建 session、
  用 `WithSession(ctx)` 挂上,并装 `sessionWriter` 在首次 `WriteHeader` /
  `Write` **之前** commit。

## 3. 约束(禁止破坏)

- **`Set-Cookie` 必须先于 body**。`sessionWriter.commit()` 由 `WriteHeader`
  与 `Write` 触发,handler 未写响应时中间件退出前再兜底一次。最多执行一次
  (`committed` 守卫)。首次写之后的属性变更不落盘,与任何 header 同约束。
- **惰性 id + 惰性 store entry**。全新且未改过的 session 保持原样
  (`!modified && !hadID` → 不发 id、不 `Save`、不发 cookie)。匿名流量绝不
  能分配 session。
- **滑动续期**。任何携带 session 的请求都要 `Save` 一次以刷新 store TTL 和
  cookie `Max-Age`,即便属性没改(仅 `hadID` 就触发持久化)。
- **RenewID = 删除 + 重生**。`renew && hadID` 时先从 store 删旧 id 再生成新
  id;属性保留。这是防会话固定的保证。
- **Cookie 恒为 `HttpOnly`**。不可配——JS 可读的 session cookie 基本是 bug,
  且 bool 零值区分不了显式 `false` 与"未设"。
- **id 熵**:`crypto/rand` 32 字节,base64url 编码。
- **远端后端不走注册表**。活 Redis client 不该跨测试/进程重启塞进包级 map;
  以 `SessionStore` bean 形式贡献。注册表只留给进程静态默认值(如 `Memory`)。

## 4. 权衡 / 未做的方案

- **`ByteStore` 序列化用 JSON,不用 gob**:跨语言可读、无版本意外,天然保属
  性 JSON 友好。
- **首次写后的 `Session` 变更静默丢失,不报错**。与任何晚发 header 的静默丢
  失同性质。文档说明即可;调用方尽早改属性。
- **响应中途 `Save` 失败,新会话不发 cookie**。宁可不发 cookie,也不给客户端
  一个 store 里没有的 id;请求余下部分正常完成。
- **分布式后端做 bean 不做注册表条目**。与 `starter-lock-redis` 一致——活
  client 属于容器,不属于包级全局 map。
