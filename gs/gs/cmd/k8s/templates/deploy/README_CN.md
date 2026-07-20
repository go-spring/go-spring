# 部署 — GS_PROJECT_NAME

[English](README.md) | [中文](README_CN.md)

由 `gs k8s` 生成的 Kubernetes 部署脚手架。这里的一切都是**可改的起点**,
不是运行时依赖——请按你的集群调整。

## 目录结构

```
Dockerfile                 # 多阶段构建 -> distroless/static:nonroot
conf/app-k8s.properties    # k8s profile:JSON 日志到 stdout、drain 窗口
deploy/k8s/                # (默认) Kustomize
  base/                    # Deployment、Service、HPA、ServiceMonitor
  overlays/dev             # 1 副本、dev- 前缀、:dev 镜像
  overlays/prod            # 3 副本、更紧的资源
  monitoring/              # prometheus-adapter + 自定义指标 HPA 样例
deploy/helm/               # (gs k8s --format helm) Helm chart
  Chart.yaml、values.yaml
  templates/               # deployment、service、hpa、servicemonitor、adapter
```

`gs k8s` 只生成**一种**打包格式。默认是 Kustomize(`deploy/k8s`);
`gs k8s --format helm` 改为生成 Helm chart(`deploy/helm`)。

## 构建与部署

```sh
docker build -t GS_IMAGE:latest .

# Kustomize:
kubectl apply -k deploy/k8s/overlays/dev     # 或 overlays/prod

# Helm:
helm install GS_APP_NAME deploy/helm         # 或:helm template deploy/helm | kubectl apply -f -
```

无集群时试运行:

```sh
kubectl kustomize deploy/k8s/base            # 渲染 Kustomize manifest
kubectl apply -k deploy/k8s/overlays/dev --dry-run=client
helm template deploy/helm                    # 渲染 Helm manifest
```

## 健康探针

探针全部命中 actuator **管理端口(GS_MGMT_PORT)**:

- `startupProbe` → `/startup` —— 应用启动完成后才通过,慢启动不会被 liveness 误杀。
- `livenessProbe` → `/health` —— 只看进程存活;依赖降级不会触发重启。
- `readinessProbe` → `/readiness` —— 优雅停机期间返回 503,先把 Pod 从 Service
  endpoints 摘掉再停 server。

### 启动预算调优

startupProbe 的**预算** ≈ `periodSeconds × failureThreshold`。默认值
(`3 × 40 = 120s`)故意留得宽。只有它通过后,liveness / readiness 才接管。

决定冷启动的**不是 bean 装配**(几百个 bean 的反射接线是亚毫秒级),而是各
starter **有意为之**的启动期 I/O fail-fast:DB/Redis 拨号、配置中心首拉、
服务发现 informer sync,以及——如果用了 `starter-migration-gorm`——schema 迁移。
按你的 **p99 启动**、而非 p50 来定预算。

| 应用画像 | 启动耗时主要来自 | `periodSeconds` | `failureThreshold` | 预算 |
|---|---|---|---|---|
| 小型(少量 starter、无迁移) | 进程初始化 + 少数拨号 | 3 | 10 | ~30s |
| 典型微服务(DB + Redis + 配置中心 + 发现) | 拨号 + 配置首拉 + informer sync | 5 | 12 | ~60s |
| 大型(依赖多、informer、缓存预热) | 大量拨号 + 预热 | 5 | 24 | ~120s |
| 启动时跑 DB 迁移 | 迁移 DDL(可能几分钟) | 10 | ≥ 最长迁移 ÷ 10 | ≥ 300s |

经验法则:

- **启动预算宁可给多**——启动快时它零成本(探针首次成功即通过)。预算太紧会把
  合法的慢启动(冷 DB、大迁移)拖进 CrashLoop。
- `periodSeconds` 保持小(3–5s),让快启动被尽快检测到;用 `failureThreshold`
  放大预算,而不是拉长 `initialDelaySeconds`。
- 不要把 readiness 混进 startup:`readinessProbe` 应门控在 `health.Indicator`
  (如 DB 可达)上,让流量只在依赖就绪后才进来。startup 只负责防止慢启动被
  liveness 误杀。

## 优雅停机

`terminationGracePeriodSeconds`(30s)与容器 `preStop` sleep(5s)和
`conf/app-k8s.properties` 里的 `app.shutdown.timeout` / `app.shutdown.pre-stop-delay`
对齐。滚动更新无损排空在途请求。

## 指标

`servicemonitor.yaml` 抓取管理端口的 `/metrics`,需要 Prometheus Operator。
只有应用接了 **starter-otel** 才会暴露 `/metrics`,否则请删除该 ServiceMonitor。

## 基于自定义指标的扩缩容

默认 HPA 按 CPU 扩缩。要按业务指标(如每秒请求数)扩缩,链路是:

```
actuator /metrics -> ServiceMonitor -> Prometheus -> prometheus-adapter
  -> custom.metrics.k8s.io -> HPA
```

- **Kustomize**:安装 prometheus-adapter,把
  `deploy/k8s/monitoring/prometheus-adapter-configmap.yaml` 应用到它所在的
  namespace,然后用 `deploy/k8s/monitoring/hpa-custom-metric.yaml` **替换**
  `base/hpa.yaml`(两者同时存在会让 Deployment 挂两个 autoscaler)。
- **Helm**:设 `autoscaling.customMetric.enabled=true`(HPA 增加一个 Pods
  指标);若想让 chart 一并渲染 adapter 规则,再设 `prometheusAdapter.enabled=true`。

adapter ConfigMap 把 `http_requests_total` 映射成每 Pod 的
`http_requests_per_second`;请按你应用的实际指标调整 series 与查询。

## Pod 元数据

Deployment 通过 Downward API 注入 Pod 字段(`GS_POD_NAME`、`GS_POD_NAMESPACE`、
`GS_POD_IP`、`GS_NODE_NAME`、`GS_POD_SERVICE_ACCOUNT`),并把 labels/annotations
挂载到 `/etc/podinfo`。在应用里用 `go-spring.org/spring/podinfo` 读取:

```go
gs.Object(&podinfo.PodInfo{})
```
