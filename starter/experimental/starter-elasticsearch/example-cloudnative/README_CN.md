# starter-elasticsearch 云原生示例

围绕 Elasticsearch 客户端组合云原生能力集的 starter-elasticsearch 旗舰示例——**服务发现**、**韧性(resilience)**、**健康检查**、**动态配置**。

## 能力

- **服务发现**:节点地址通过已注册的发现后端(`service-name`)解析,而非硬编码配置——只有 解析→拨号→服务 全部成功,index round-trip 才会成功。
- **韧性**:开启 `resilience.enabled` 后,每个请求都通过内置 `"default"` executor;超过 `rate-limit` 的突发被以 `ErrRateLimited` 拒绝。韧性配置本身是 `gs.Dync` 字段,可热更新。
- **健康检查**:每实例的 elasticsearch `health.Indicator` 被 `starter-actuator` 在 `:9370` 聚合——`/readyz` 反映集群状态。
- **动态配置**:`gs.Dync[string]` label 绑定到一个被监听的配置文件;编辑后无需重启即可热更新。

## 布局

```
main.go               旗舰应用(容器 + 自测)
conf/app.properties      发现、韧性、actuator 配置
check.sh                 docker-gated 冒烟测试
```

## 手动验证

需要一个本地 Elasticsearch(`docker compose up -d`):

```bash
cd starter/experimental/starter-elasticsearch/example-cloudnative
docker compose up -d
go run . -manual
```

另开终端:

```bash
curl http://127.0.0.1:9370/readyz    # 健康检查(elasticsearch 集群)
curl 'http://127.0.0.1:9200'         # 该集群经发现后端解析
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 拉起 Elasticsearch,然后断言健康探针为 UP、经发现解析的 index/get round-trip 成功、突发部分被以 `ErrRateLimited` 拒绝;退出码 0 表示通过。缺少 docker 时优雅跳过。
