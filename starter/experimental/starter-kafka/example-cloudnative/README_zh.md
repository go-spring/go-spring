# starter-kafka 云原生示例

围绕 Kafka(franz-go)客户端组合云原生能力集的 starter-kafka 旗舰示例——以 **生产→消费 round-trip** 作为客户端操作,受 **韧性(resilience)** 保护,并具备 **健康检查** 与 **动态配置**。

## 能力

- **Round-trip**:客户端操作是同步 produce 穿过 broker,再从同一 topic 消费回来——MQ 原型,端到端证明 broker 可达。
- **韧性**:开启 `spring.kafka.instances.a.resilience.enabled` 后,每条同步 produce 都经由 `GuardedProduceSync` → 内置 `"default"` executor;超过 `rate-limit` 的突发被以 `ErrRateLimited` 拒绝。消费不受保护(franz-go 的 poll 路径是被动的)。
- **健康检查**:starter 本身未注册 `health.Indicator`,故应用自行导出一个(对 broker 的 `Ping` 探针),并由 `starter-actuator` 在 `:9370` 聚合——`/readyz` 反映 broker 可达性。
- **动态配置**:`gs.Dync[string]` 字段绑定到被监听的文件;`file-watch` provider 无需重启即可热更新。
- **可观测性**:kotel 的 span/指标 + observe 访问日志 hook 依托 `starter-otel` 安装的 OTel 全局。

发现(discovery)有意省略:Kafka 用 bootstrap broker(`spring.kafka.instances.a.brokers`)给客户端做种子,不存在需要解析的 service-name。

## 布局

```
example.go               旗舰应用(容器 + 自测)
conf/app.properties      kafka、韧性、actuator 配置
check.sh                 docker-gated 冒烟测试
docker-compose.yml       单节点 KRaft Kafka
```

## 手动验证

需要一个本地 Kafka(`docker compose up -d`):

```bash
cd starter/experimental/starter-kafka/example-cloudnative
docker compose up -d
go run . -manual
```

另开终端:

```bash
curl http://127.0.0.1:9370/readyz                 # 健康检查(broker Ping 探针)
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 拉起单节点 KRaft Kafka,然后断言 actuator 探针为 UP、生产→消费 round-trip 成功、突发部分被以 `ErrRateLimited` 拒绝、动态配置 label 热更新生效;退出码 0 表示通过。缺少 docker 时优雅跳过。
