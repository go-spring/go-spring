# starter-gateway 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-gateway` 属于 **Server** 形态(见
[starter/DESIGN.md](../DESIGN.md) §2.1),以独立端口(默认 `:9440`)运行独立
API 网关。它以 Go 惯用法落地 Spring Cloud Gateway 的 Route/Predicate/Filter
模型:Predicate=`func(*http.Request) bool`,Filter=`func(next http.Handler)
http.Handler`,路由即函数组合,不做运行时 DSL。

## 1. 职责与边界

- **在范围内:**路由表绑定 + 热更新;`lb://` 上游经 `stdlib/discovery` +
  `stdlib/loadbalance`;`FilterWrapper` 缝隙让 jwt-auth、lua 等可插拔 filter
  免硬 import 挂载。
- **不在范围内:**运行时 DSL / 规则引擎、控制面同步、L4/TCP 代理。韧性
  (重试 / 熔断 / 限流)交给 `stdlib/resilience`,不重造。

## 2. 关键抽象

- **RouteTable / GatewayServer。**两个 bean,分别绑定到 `${spring.gateway}`
  与 `${spring.gateway.server}`,用**字段级 `value:"..."` 标签**注入——
  **不要**用 `gs.TagArg("${prefix}")` 注入整个结构体,那会报
  "property is not a simple value"。
- **`FilterWrapper` 缝隙。**单方法本地接口
  (`Wrap(next http.Handler) http.Handler`),经 `map[string]FilterWrapper
  autowire:"?"` 收集。`starter-security-jwt`、`starter-lua-filter` 注册
  满足该形状的 bean;gateway 从不 import 它们。
- **`lb://<service>`。**上游 URL 前缀,通过
  `discovery.NewLiveDialer` + `loadbalance.Pool.Pick` 解析——与其它 client
  starter 用同一套客户端栈。mesh 模式在其中集中退化,无需网关侧分支。

## 3. 约束

- **warmup 阶段编译,而非 eager 编译。**`Wrappers map[string]FilterWrapper
  autowire:"?"` 在构造后才填充,故路由编译推迟到 `GatewayServer.Run` 的
  `warmup()`;坏配置仍在启动即失败(同 `starter-scheduler` 模式),只是稍后
  触发。
- **`gs.Server` bean 必须 `.Name("gatewayServer")`。**容器已有名为
  `__default__` 的默认 web-server bean,两个未命名的 `gs.Server` 会因
  duplicate 冲突。
- **`gs.Dync[map]` 默认值必须为空。**写 `${routes:=}`,**不要**写
  `${routes:={}}`——bind 层会拒绝非空 map 默认值:
  "map can't have a non-empty default value"。
- **热更新靠 map 指针对比。**刷新循环比较
  `reflect.ValueOf(m).Pointer()` 与上次观察值;变化时 recompile。编译失败
  保留旧路由表,不落成损坏路由。
- 内部依赖靠 `go.work` 解析;不跑 `go mod tidy`。

## 4. 取舍 / 弃选方案

- **不做运行时 DSL / 规则引擎。**Predicate 与 Filter 是 Go 函数。DSL 会加一层
  解析器与第二套执行模型,而热更新已由 Go 二进制 + gs-http-gen 风格的
  config 刷新提供。
- **不硬 import jwt-auth / lua 模块。**`FilterWrapper` 缝隙保持网关与
  `starter-security-jwt`、`starter-lua-filter` 解耦,应用通过空导入选择要
  启用的 filter——与 WebSocket 族的实现切换一样。
- **`gs.Server`(自有端口)而非 Contributor 挂到主 web server。**网关是有
  独立端口、超时、生命周期的独立进程关注点;与业务端口复用会让灰度、TLS
  终止、流量隔离变复杂。
