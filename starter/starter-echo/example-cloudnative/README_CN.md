# starter-echo 云原生示例

一个自包含的 starter-echo 旗舰示例,在单个应用里组合了完整的云原生能力集——**健康检查**、**服务发现**、**韧性(resilience)**、**可观测性**、**动态配置**——无需任何外部服务或 docker。

## 能力

- **健康检查**:一个 `health.Indicator` bean 被 `starter-actuator` 在其管理端口(`:9370`)上聚合。切换它会让 `/readyz` 在 `200` 与 `503` 之间翻转,而 `/health` 保持可用。
- **服务发现**:`discovery.NewStaticDiscovery` 后端给出应用自己的 echo 地址(`:8082`);应用在客户端解析它并端到端拨号。
- **韧性**:内置 `"default"` 驱动对任意函数(`Executor.Execute` → `ErrRateLimited`)和入站路由(`/limited` → `429`)分别做限流。
- **动态配置**:一个 `gs.Dync[string]` 字段绑定到被监视的文件(`file-watch`)。像 kubelet 切换 ConfigMap 那样编辑它,可把新值热更新进 `/greeting`。
- **可观测性**:starter-echo 的 Tracing/Metrics/AccessLog 中间件默认开启,依托 OTel 全局。

## 布局

```
main.go               旗舰应用(容器 + 自测)
conf/app.properties      端口、actuator、file-watch imports
check.sh                 零依赖冒烟测试
```

| 端口 | 归属 |
|------|------|
| `:8082` | starter-echo 业务路由(`/`、`/greeting`、`/limited`) |
| `:9370` | starter-actuator 探针(`/health`、`/readyz`、`/startup`、`/info`) |

## 手动验证

```bash
cd starter/starter-echo/example-cloudnative
go run . -manual
```

另开终端,逐项验证:

```bash
curl http://127.0.0.1:9370/readyz                 # 健康检查
curl http://127.0.0.1:8082/greeting               # 动态配置(编辑 ./mount 触发热更新)
for i in $(seq 1 15); do curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8082/limited; done
curl http://127.0.0.1:8082/                       # 服务发现解析应用自身
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行应用并等待自测断言全部能力;退出码 0 表示通过。
