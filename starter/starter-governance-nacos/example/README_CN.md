# starter-governance-nacos 示例

演示 starter-governance-nacos 的 Nacos 治理规则源。

## 功能验证

- **冷加载**：启动时从专属 dataId 读取规则文档，种下初始治理策略
- **规则热推送**：向该 dataId 发布新文档，监听器实时送达、策略即时翻转——无需重启，也不触发应用级属性刷新
- **治理隔离**：规则文档归治理中心自有，启用治理不会开启应用配置热更新

> 需要 `127.0.0.1:8848` 上的 Nacos——`docker-compose.yml` 可一键拉起（冒烟脚本会自动做）。

## 手动验证

```bash
cd starter-governance-nacos/example
docker compose up -d
go run . -manual
```

应用保持运行，每秒打印一次当前生效的策略：

```
policy: enabled=true timeout=100ms retries=2
```

另开一个终端，发布新文档，观察策略翻转为 900ms：

```bash
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
  --data-urlencode 'dataId=gs-govern-demo.yaml' \
  --data-urlencode 'group=DEFAULT_GROUP' \
  --data-urlencode 'content=spring:
  governance:
    enabled: true
    client:
      default:
        enabled: true
        attempt-timeout: 900ms
        max-retries: 2'
# 打印: policy: enabled=true timeout=900ms retries=2
```

`Ctrl+C` 停止应用；`docker compose down -v` 回收 Nacos。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 用 docker compose 拉起 Nacos 后运行示例（不带 `-manual`）：种下 100ms
文档、断言冷加载、发布 900ms 文档并验证热推送，退出码 0 即通过。
