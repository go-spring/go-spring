# 云原生旗舰示例

一个自包含的应用,组合了 go-spring 提供的四项横切云原生能力:**健康检查**、**服务发现**、**韧性(resilience)**、**动态配置**。无需任何外部服务或 docker——应用用一个指向自身的静态发现后端做解析,并对本地被监视文件做热更新。

## 能力

- **健康检查**:一个 `health.Indicator` bean 被 `starter-actuator` 在其管理端口(`:9370`)上聚合。切换该指标会让 `/readyz` 在 `200` 与 `503` 之间翻转,而 `/health` 保持可用(降级依赖绝不触发 liveness)。
- **服务发现**:`discovery.NewStaticDiscovery` 后端给出应用自己的 gin 地址(`:8081`);应用在客户端解析它并端到端拨号访问返回的端点。
- **韧性**:内置 `"default"` 驱动对任意函数(`Executor.Execute` → `ErrRateLimited`)和入站 HTTP 路由(`/limited` → `429`)分别做限流。
- **动态配置**:一个 `gs.Dync[string]` 字段绑定到被监视的文件(`file-watch` provider)。像 kubelet 切换 ConfigMap 那样(原子 `..data` 软链)编辑该文件,无需重启即可把新值热更新进 `/greeting` 处理器。

## 布局

```
main.go               旗舰应用(容器 + 自测)
conf/app.properties   端口、actuator、file-watch imports
check.sh              零依赖冒烟测试
```

| 端口 | 归属 |
|------|------|
| `:8081` | starter-gin 业务路由(`/`、`/greeting`、`/limited`) |
| `:9370` | starter-actuator 探针(`/health`、`/readyz`、`/startup`、`/info`) |

## 手动验证

终端 1 —— 启动应用:

```bash
cd examples/cloudnative
go run . -manual
```

终端 2 —— 逐项验证:

```bash
# 健康检查:就绪聚合
curl http://127.0.0.1:9370/readyz

# 动态配置:当前问候语
curl http://127.0.0.1:8081/greeting
# 编辑 ./mount/application.properties -> demo.greeting=hello-2
# 再次 curl;无需重启值已变化

# 韧性:突刺限流路由 -> 出现 429
for i in $(seq 1 15); do curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8081/limited; done

# 服务发现:应用在客户端解析自己的 gin 地址
curl http://127.0.0.1:8081/
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行应用并等待自测断言全部四项能力;退出码 0 表示通过。
