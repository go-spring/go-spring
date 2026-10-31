# starter-governance-etcd 示例

演示 starter-governance-etcd 的 etcd 治理规则源。

## 功能验证

- **冷加载**：启动时从专属 etcd key 读取规则文档，种下初始治理策略
- **规则热推送**：向该 key 发布新文档，watch 流实时送达、策略即时翻转——无需重启，也不触发应用级属性刷新
- **治理隔离**：规则文档归治理中心自有，启用治理不会开启应用配置热更新

> 需要 `127.0.0.1:2379` 上的 etcd——`docker-compose.yml` 可一键拉起（冒烟脚本会自动做）。

## 手动验证

```bash
cd starter-governance-etcd/example
docker compose up -d
go run . -manual
```

应用保持运行，每秒打印一次当前生效的策略：

```
policy: enabled=true timeout=100ms retries=2
```

另开一个终端，发布新文档，观察策略翻转为 900ms：

```bash
docker exec starter-governance-etcd etcdctl put /gs-govern-demo.yaml 'spring:
  governance:
    enabled: true
    client:
      enabled: true
      resilience:
        timeout: 900ms
      max-retries: 2'
# 打印: policy: enabled=true timeout=900ms retries=2
```

`Ctrl+C` 停止应用；`docker compose down -v` 回收 etcd。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 用 docker compose 拉起 etcd 后运行示例（不带 `-manual`）：种下 100ms
文档、断言冷加载、发布 900ms 文档并验证热推送，退出码 0 即通过。
