# starter-neo4j 云原生示例

实验性 **starter-neo4j** 的旗舰示例，围绕图数据库客户端组合云原生能力集——
**服务发现**、**弹性容错**、**健康检查** 与 **可观测性**。

## 能力

- **服务发现**：neo4j 地址通过已注册的 discovery 后端（`service-name`）解析，
  而非硬编码配置。由于 neo4j 驱动没有 dialer 注入点，解析在启动时一次性完成——
  Cypher 往返仅当 resolve → dial → serve 全部成功时才成立。
- **弹性容错**：开启 `resilience.enabled` 后，经由 `StarterNeo4j.Query` /
  `StarterNeo4j.RunWithResilience` 的查询会经过内置 `"default"` executor；
  超过 `rate-limit` 的突发流量以 `ErrRateLimited` 拒绝。容错配置为 `gs.Dync`
  字段，支持热更新。
- **健康检查**：每实例的 neo4j `health.Indicator` 由 `starter-actuator` 聚合到
  `:9370`——`/readyz` 反映服务器连通性。
- **动态配置**：`gs.Dync[string]` 字段绑定被监听的文件；编辑后无需重启即可
  热加载新值。
- **可观测性**：`StarterNeo4j.Query` 经模块内建埋点在 OTel 全局变量上产出
  span、`db.client.*` 指标与访问日志。

## 目录结构

```
example.go              旗舰应用（容器 + 自测）
conf/app.properties     discovery、resilience、actuator 配置
check.sh                docker 门控的冒烟测试
```

## 手动测试

需要本地 Neo4j（`docker compose up -d`）：

```bash
cd starter/experimental/starter-neo4j/example-cloudnative
docker compose up -d
go run . -manual
```

在另一个终端：

```bash
curl http://127.0.0.1:9370/readyz                 # 健康检查（neo4j 服务器）
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 拉起 Neo4j，随后断言健康检查探针为 UP、
经 discovery 解析的 Cypher 往返成功，以及突发流量部分被 `ErrRateLimited`
拒绝；退出码 0 表示通过。docker 不可用时优雅跳过。
