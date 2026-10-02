# starter-mongodb 云原生示例

围绕 MongoDB 客户端组合云原生能力集的 starter-mongodb 旗舰示例——**服务发现**、**韧性(resilience)**、**健康检查**、**动态配置**。

## 能力

- **服务发现**:`disc` 实例的地址通过已注册的发现后端(`service-name=mongo-cluster`)解析,而非硬编码配置——只有 解析→拨号→服务 全部成功,round-trip 才会成功。
- **韧性**:开启 `resilience.enabled` 后,每次连接拨号都通过内置 `"default"` executor;超过 `rate-limit` 的新连接突发被以 `ErrRateLimited` 拒绝。与 go-redis(逐命令 hook)不同,MongoDB 没有逐命令 hook,因此接缝在**拨号层**:保护的是建连,而非已打开连接的每次操作。韧性配置本身是 `gs.Dync` 字段,可热更新。
- **健康检查**:每实例的 mongodb `health.Indicator` 被 `starter-actuator` 在 `:9370` 聚合——`/readyz` 反映客户端连接池状态。
- **动态配置**:`gs.Dync[string]` 标签绑定到被监听的文件;编辑后无需重启即可热更新。

## 布局

```
main.go               旗舰应用(容器 + 自测)
conf/app.properties      mongodb、发现、韧性、actuator 配置
check.sh                 docker-gated 冒烟测试
```

## 手动验证

需要一个本地 MongoDB(`docker compose up -d`):

```bash
cd starter/experimental/starter-mongodb/example-cloudnative
docker compose up -d
go run . -manual
```

另开终端:

```bash
curl http://127.0.0.1:9370/readyz                 # 健康检查(mongodb 客户端)
# "disc" 实例经发现后端解析(mongo-cluster)
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 拉起 MongoDB,然后断言健康探针为 UP、直接 CRUD round-trip 成功、经发现解析的 round-trip 成功、新连接突发部分被以 `ErrRateLimited` 拒绝;退出码 0 表示通过。缺少 docker 时优雅跳过。
