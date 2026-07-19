# 全栈参考应用

一个端到端示例,证明 Go-Spring 的各个 starter 能在同一条请求链路上协同工作:
**网关 → 订单服务(A) → 库存服务(B)**,一次性把服务发现、配置中心、分布式事务、
可观测性和安全全部接线到一起。

它的真正价值在于充当一个**集成测试台**:把这么多 starter 装配到同一处,才会暴露出
只有在所有东西一起运行时才出现的冲突(端口分配、停机排空顺序、trace 传播缺口、
配置前缀)。这些发现都记录在 [INTEGRATION_NOTES_CN.md](INTEGRATION_NOTES_CN.md)。

> 只消费既有 starter,这里不引入任何新 starter。

## 拓扑

三个独立进程,一个 Go module(`fullstack`),每个 `cmd/` 目录产出一个二进制:

| 进程      | 框架                     | 业务端口      | 管理端口 | 注册名       | 角色 |
|-----------|--------------------------|---------------|----------|--------------|------|
| gateway   | starter-gateway          | `:9440`       | `:9370`  | —            | 边缘,路由 `/api/**` → `lb://order` |
| order     | 内置 HTTP mux            | `:8081`       | `:9371`  | `order`      | JWT 资源服务器,运行 Saga |
| inventory | starter-gin              | `:8082`       | `:9372`  | `inventory`  | 库存预留/释放(Saga 下游) |

依赖(Docker):**Consul**(`:8500`)负责发现/注册,**Nacos**(`:8848`)负责
配置中心热更新。

## 请求流

```
client --POST /api/orders (Bearer token)--> gateway :9440
   gateway  stripPrefix(1),经 Consul 解析 lb://order -----> order :8081
      order  JWT 校验 (starter-security-jwt)
      order  Saga 步骤 1 "reserve"  --HTTP--> inventory :8082  /reserve
      order  Saga 步骤 2 "charge"   (Nacos 标志置位时失败)
             |__ 失败时:补偿 --HTTP--> inventory /release
   <-- 200 committed   (两步都成功)
   <-- 409 compensated (charge-fail=true;B 上的预留被释放)
```

- **服务发现**:网关的 `lb://order` 和订单的 `order→inventory` 调用都经由一个
  Consul 支撑的 `discovery.Discovery`(`internal/consuldisc`)解析,以名字 `consul`
  注册一次。实例通过 `starter-registry-consul` 自行注册。
- **配置中心**:`fullstack.order.charge-fail` 从 Nacos 导入到一个 `gs.Dync[bool]`。
  在线发布它会让下一笔订单进入补偿路径——无需重启。
- **分布式事务**:`starter-transaction-saga`。步骤 1 预留库存(补偿即释放);
  步骤 2 扣款。扣款失败会回滚*另一个*服务上的预留。
- **可观测性**:`starter-otel` + `starter-actuator`。每个服务在一个管理端口上暴露
  探针和 `/metrics`;日志带 `trace_id`;trace 从 order 传播到 inventory(W3C 头)。
  网关缺口见踩坑记录。
- **安全**:`starter-security-jwt`。订单服务是资源服务器;网关原样转发调用方的
  `Authorization` 头。
- **优雅停机**:收到 SIGTERM 后 readiness 翻为 `OUT_OF_SERVICE`,框架先排空再停
  server(`app.shutdown.pre-stop-delay`)。

## 运行

```bash
# 在本目录下。拉起 Consul + Nacos,启动三个服务,
# 驱动整条请求链路,断言 committed + compensated + 排空。
./check.sh
```

或手动运行:

```bash
docker-compose up -d                     # consul + nacos
go run ./cmd/inventory &                 # :8082
go run ./cmd/order &                     # :8081
go run ./cmd/gateway &                   # :9440

TOKEN=$(go run ./cmd/mint alice user)
curl -i -X POST http://127.0.0.1:9440/api/orders                       # 401
curl -i -X POST http://127.0.0.1:9440/api/orders -H "Authorization: Bearer $TOKEN"  # 200 committed

# 在 Nacos 里翻转配置,再下一单 -> 409 compensated:
curl -X POST http://127.0.0.1:8848/nacos/v1/cs/configs \
  --data-urlencode dataId=gs-fullstack-order \
  --data-urlencode group=DEFAULT_GROUP \
  --data-urlencode 'content=fullstack.order.charge-fail=true'
curl -i -X POST http://127.0.0.1:9440/api/orders -H "Authorization: Bearer $TOKEN"  # 409 compensated
```

## 目录结构

```
cmd/gateway     边缘;注册 Consul discovery,空导入 starter-gateway
cmd/order       JWT 资源服务器 + Saga 协调者 + Nacos 驱动的标志位
cmd/inventory   gin 服务;reserve/release;注册进 Consul
cmd/mint        打印一个 HS256 bearer token(共享密钥,无外部 IdP)
internal/consuldisc   Consul 支撑的客户端侧 discovery.Discovery
internal/authsecret   共享的 HMAC 密钥
```

另见:[English](README.md) · [踩坑记录](INTEGRATION_NOTES_CN.md)
