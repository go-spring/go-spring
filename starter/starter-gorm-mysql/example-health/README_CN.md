# starter-gorm-mysql 健康检查示例

演示 starter-gorm-mysql 的**健康集成**:每个配置的实例都会得到一个 `health.Indicator`(连接池存活),并被零配置地折入 starter-actuator 的 `/readyz` 聚合。

## 特性

- **自动健康指标**:starter 为每个实例导出一个 `health.Indicator`——无需任何应用代码。
- **actuator 聚合**:管理端口(`:9370`)上的 `/health`、`/readyz`、`/startupz` 反映数据库连接池的健康状况。

## 手动验证

需要一个本地 MySQL(`docker compose up -d`):

```bash
cd starter/starter-gorm-mysql/example-health
docker compose up -d
go run . -manual
```

然后探测管理端口:

```bash
curl http://127.0.0.1:9370/readyz
curl http://127.0.0.1:9370/health
```

数据库健康时两者都返回 `UP`。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 拉起 MySQL,然后断言 DB 可 ping 且 actuator 探针返回 UP;退出码 0 表示通过。缺少 docker 时优雅跳过。
