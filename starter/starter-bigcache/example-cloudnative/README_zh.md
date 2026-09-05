# starter-bigcache 云原生示例

围绕**进程内**缓存组合云原生能力集的 starter-bigcache 旗舰示例——**健康检查**、**韧性(resilience)**、**动态配置**、**可观测性**。由于 bigcache 没有外部服务器,本示例**完全自包含**:无需 docker、无需 docker-compose、无需对远程后端做服务发现(应用不拨号任何外部地址)。

## 能力

- **健康检查**:每实例的 bigcache `health.Indicator` 由 starter 自动导出,并被 `starter-actuator` 在 `:9370` 聚合——`/readyz` 与 `/health` 报告 UP(进程内实例存在即表示就绪)。
- **韧性**:开启 `resilience.enabled` 后,每次 `Get`/`Set`/`Delete` 都通过内置 `"default"` executor;超过 `rate-limit` 的突发被以 `ErrRateLimited` 拒绝。韧性配置本身是 `gs.Dync` 字段,策略可热更新。
- **动态配置**:`gs.Dync[string]` 字段绑定到被监视文件的 `demo.label`;修改文件即可热更新值,无需重启。
- **可观测性**:bigcache 没有插件钩子,因此 starter 的 `*starter 的 `*Cache` 包装器承载了每次操作的 span+metric+log,以及依托 OTel 全局的缓存统计 gauge(hits/misses/collisions)。
- **服务发现**:不适用——bigcache 是进程内缓存,没有需要解析的外部地址。

## 布局

```
example.go               旗舰应用(容器 + 自测)
conf/app.properties      bigcache 实例、韧性、actuator、导入配置
check.sh                 自包含冒烟测试(go run + 60s 看门狗)
```

## 手动验证

无需任何外部服务,直接运行:

```bash
cd starter/starter-bigcache/example-cloudnative
go run . -manual
```

另开终端:

```bash
curl http://127.0.0.1:9370/readyz                 # 健康检查(bigcache indicator)
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 在 60s 看门狗下运行示例;应用自行断言健康探针为 UP、进程内 Set/Get/Delete round-trip 成功、突发部分被以 `ErrRateLimited` 拒绝;退出码 0 表示通过。
