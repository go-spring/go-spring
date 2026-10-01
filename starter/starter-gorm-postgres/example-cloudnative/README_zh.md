# starter-gorm-postgres 云原生示例

围绕存储客户端组合云原生能力集的 starter-gorm-postgres 旗舰示例——**服务发现**、**韧性(resilience)**、**健康检查**、**动态配置**、**可观测性**。

## 能力

- **服务发现**:DB 地址通过已注册的发现后端(`service-name`)解析,而非硬编码配置——只有 解析→拨号→服务 全部成功,查询才会成功。
- **韧性**:开启 `resilience.enabled` 后,每条查询都通过内置 `"default"` executor;超过 `rate-limit` 的突发被以 `ErrRateLimited` 拒绝。
- **健康检查**:每实例的 gorm `health.Indicator` 被 `starter-actuator` 在 `:9370` 聚合——`/readyz` 反映连接池状态。
- **动态配置**:一个 `gs.Dync[string]` 字段绑定到被监视的文件(`file-watch`);编辑它可无重启热更新。
- **可观测性**:gorm observe 插件为每次操作声明身份,resilience 层通过 OTel 全局发射 span、指标与访问日志。

## 布局

```
example.go               旗舰应用(容器 + 自测)
conf/app.properties      发现、韧性、actuator、file-watch 配置
check.sh                 docker-gated 冒烟测试
```

## 手动验证

需要一个本地 PostgreSQL(`docker compose up -d`):

```bash
cd starter/starter-gorm-postgres/example-cloudnative
docker compose up -d
go run . -manual
```

另开终端:

```bash
curl http://127.0.0.1:9370/readyz                 # 健康检查(db 连接池)
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 拉起 PostgreSQL,然后断言健康探针为 UP、经发现解析的查询成功、突发部分被以 `ErrRateLimited` 拒绝、被监视文件可热更新;退出码 0 表示通过。缺少 docker 时优雅跳过。
