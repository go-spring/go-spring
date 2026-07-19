# 集成踩坑记录

把这么多 starter 装配进同一条请求链路后浮现出来的问题。每一条要么是
**已在示例中解决**(附解决方式),要么是**反哺回某个 starter 的缺口**。这是本参考
应用的首要产出——一份回归基线,也是一份生态的待办清单。

## 1. 多进程管理端口冲突 —— *已解决*

每个服务都空导入 `starter-actuator`,默认绑 `:9370`,而且各自还想要一个业务端口。
在一台主机上跑三个服务,就意味着三个进程争抢同一个 `:9370`(以及内置 HTTP 的
`:9090`)。这个问题在单服务示例里是不可见的。

**已解决**,办法是给每个进程分配不同端口:
`gateway 9440/9370`、`order 8081/9371`、`inventory 8082/9372`。没有自动偏移;
生态默认一个 pod 一个服务,所以多服务主机必须显式分配。已在各自的
`conf/app.properties` 中记录。

## 2. Consul 没有客户端侧发现 starter —— *缺口反哺*

`starter-registry-consul` 只做注册侧(它宣告一个实例)。网关的 `lb://order` 和
order→inventory 需要一个*客户端侧*的 `discovery.Discovery`,而只有
`starter-discovery-k8s` 提供这种能力。

**在示例中桥接**,通过 `internal/consuldisc`——一个由 Consul catalog 支撑的
`discovery.Discovery`,用 `discovery.Register("consul", …)` 注册。它证明了
`stdlib/discovery` 抽象足以填补这个缺口,但一个一等公民 `starter-discovery-consul`
可以省掉这段每应用重写的代码。**反哺**:候选新 starter。

## 3. `lb://` 经全局注册表而非 bean 解析 —— *顺序注意点*

网关经由 `discovery.MustGet(name)` 解析 `lb://` 上游——用的是进程级全局的
`stdlib/discovery` 注册表,而不是 IoC 容器。所以 `discovery.Register(...)` 必须在
`init()` 里跑(先于路由编译),而且它是一个全局副作用,不是可注入的 bean。把全局
注册表和基于 bean 的接线混在一起很容易出错;示例把 `Register` 调用放在每个 `main`
的 `init` 里,紧挨着空导入,让顺序一目了然。

## 4. Trace 熬不过网关这一跳 —— *缺口反哺*

`starter-otel` 设置了全局 W3C 传播器,所以 order→inventory 的连续性靠调用两侧手动
`Inject`/`Extract` 就能工作。但**网关不会把 trace 传播**给 `order`:
`httputil.ReverseProxy`(在 `starter-gateway` 里)不注入 `traceparent`,网关也没在
转发这一跳加任何客户端埋点。结果是网关有自己的一条 trace,而 `order` 开了一条全新
的根 trace——端到端的 trace 在边缘断开了。

**反哺**:`starter-gateway` 应当在转发这一跳贡献 otel 传播(以及一个 server/client
span),就像各服务手动做的那样。在那之前,"一次请求 = 一条 trace"只在 `order`
往里才成立。

## 5. 入站 span 靠手动 —— 双重埋点风险 —— *观察项*

`starter-gin` 和内置 HTTP server 都不会起 server span,所以每个服务自己加了一个
span 中间件(`traceMiddleware` / `traceServer`)才能把 `trace_id` 打进日志。这在
**当下**是对的:`starter-otel` 只提供 provider、不加任何自动中间件,所以每个请求
恰好一个 span。但如果未来某个版本给这些 starter 加了自动入站埋点,这层手动中间件
就会**每个请求产生两个 span**——届时需与该改动同步删掉手动这层。

两份 extract-then-start 样板已**去重**到
`StarterOTel.StartServerSpan(ctx, header, tracer, name)`
(`starter/starter-otel/serverspan.go`)——它是本 starter 已提供的出站
`discovery.SetTraceInjector` 缝隙的入站对偶。两个服务现在都调用它,这样将来若自动
入站埋点落地,只有一处要退役,而不是两处。详见下方 Task 07.9 收敛记录。

## 6. 认证放在资源服务器而非网关 —— *设计决策*

`Authenticator.Wrap` 契合网关的 `Filter` 缝隙,所以 JWT *本可以*在边缘强制执行。
示例却选择在 `order`(资源服务器)执行,让网关原样转发 `Authorization`。理由:拥有
被保护资源的服务应当自己决定授权,这也让网关保持为一个纯粹的路由器。两种都成立;
把选择记录下来,以免被误认为是限制。

## 7. Nacos `optional:` 导入是冷启动必需 —— *已解决*

`order` 从 Nacos 导入它的扣款失败标志。不加 `optional:` 前缀的话,当 data id 尚不
存在时(首次运行、任何发布之前)服务会快速失败。加上 `optional:` 后,它以默认值
启动,并在值发布后在线刷新。已确认 `gs.Dync` 字段无需重启即更新。

## 8. `Rooter __default__` 冲突 —— *因构造而规避*

已知的 `starter-config-file` 陷阱(一个 `Rooter` 桥接在 `__default__` 上冲突)在
这里**不会**出现,因为示例用的是 `starter-config-nacos`,它的 provider 不需要
`Rooter` 桥接。记录下来,以便本应用将来若切到 `starter-config-file` 时重新核查。

---

## Task 07.9 收敛记录

并行开发计划列出的五个待收敛冲突面,逐条对照上文代码给出结论:

1. **配置前缀冲突 —— 非问题。** 每种能力独占一个顶层前缀(`spring.gateway`、
   `spring.observability`、`spring.actuator`、`spring.security.jwt`),同能力多实现
   族则有意共用一个(`spring.registry`、`spring.config`、`spring.transaction`、
   `spring.redis`),换实现只改 import。没有两个不相关 starter 读同一个 key。唯一真正
   的同机冲突——端口——已由上文第 1 条解决。
2. **多 server 停机顺序 —— 已验证正确。** actuator server 是 `PreStopper`,SIGTERM
   时先翻 readiness,`pre-stop-delay` 窗口内所有 server(含 actuator)仍在服务,然后
   并发停止。server 间无需顺序保证,因为排空完成前没有任何 server 停止
   (`spring/gs/internal/gs_app/app.go`)。
3. **OTel 双重埋点 —— 非问题。** `starter-otel` 只提供 provider;
   `starter-gin`/`starter-gateway` 不加 otel 中间件;没有 provider 被装两次。每个请求
   恰好一个 span(见第 5 条)。
4. **`Rooter __default__` 冲突 —— 已加护栏。** 每个配置源 starter 的刷新桥接都取了
   不同的名字,且容器对重复 bean id 快速失败(`resolving.go` 的
   `checkDuplicateBeans`),两个配置源不会静默冲突。本例只用 Nacos 未触发,但即便切换
   也是安全的。
5. **重复 trace 辅助 —— 已提取。** 两个服务里重复的入站 extract-then-start 已移到
   `StarterOTel.StartServerSpan`——出站注入缝隙的入站对偶。不放进 `stdlib`(零三方依赖
   规则,而这里需要 OTel)。判定不值得提取的:`extractField`(仅一处)、`os.Chdir`
   工作目录 init(示例样板)、LiveDialer `http.Client`(其可复用内核
   `discovery.NewLiveDialer` 早已存在)。

**仍未闭合(反哺,超出去重范围):** 上文第 4 条——trace 熬不过网关这一跳,因为
`starter-gateway` 有意不依赖 otel、也不起 span。现已具备无需让网关耦合 OTel 即可闭合
的缝隙:用 `discovery.TraceRoundTripper` 包裹代理 transport(转发跳注入,网关已导入
`discovery`),并给网关加一个入站 span。作为 `starter-gateway` 增强项跟踪,不属于本次
收敛修复。

---



- 无 token 的 `POST /api/orders` → 在 `order` 处 **401**。
- 带 token 的 `POST /api/orders` → **200 committed**,请求穿过
  gateway → order → inventory,经 Consul 发现。
- 向 Nacos 发布 `fullstack.order.charge-fail=true` → 下一笔订单 **409
  compensated**,且 `inventory` 打出一条对应的 `/release` —— 由一次在线配置变更触发
  的跨服务回滚。
- SIGTERM → 三个服务都排空后退出。

见 [English](INTEGRATION_NOTES.md)。
