# starter-memcached 云原生示例

围绕缓存客户端组合云原生能力集的 starter-memcached 旗舰示例——**服务发现**、**韧性(resilience)**、**健康检查**、**可观测性**。

## 能力

- **服务发现**:memcached 地址通过已注册的发现后端(`service-name`)解析,而非硬编码配置——只有 解析→拨号→服务 全部成功,round-trip 才会成功。
- **韧性**:开启 `resilience.enabled` 后,每条命令都通过内置 `"default"` executor;超过 `rate-limit` 的突发被以 `ErrRateLimited` 拒绝。韧性配置本身是 `gs.Dync` 字段,可热更新。
- **健康检查**:每实例的 memcached `health.Indicator` 被 `starter-actuator` 在 `:9370` 聚合——`/readyz` 反映集群状态。
- **可观测性**:starter 内置观测层依托 OTel 全局。

## 布局

```
example.go               旗舰应用(容器 + 自测)
conf/app.properties      发现、韧性、actuator 配置
check.sh                 docker-gated 冒烟测试
```

## 手动验证

需要一个本地 memcached(`docker compose up -d`):

```bash
cd starter/starter-memcached/example-cloudnative
docker compose up -d
go run . -manual
```

另开终端:

```bash
curl http://127.0.0.1:9370/readyz                 # 健康检查(memcached 集群)
printf 'add cn:key 0 0 5\r\nhello\r\n' | nc 127.0.0.1 11211   # 该实例经发现后端解析
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 拉起 memcached,然后断言健康探针为 UP、经发现解析的 round-trip 成功、突发部分被以 `ErrRateLimited` 拒绝;退出码 0 表示通过。缺少 docker 时优雅跳过。
