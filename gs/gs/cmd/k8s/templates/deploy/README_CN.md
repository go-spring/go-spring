# 部署 — GS_PROJECT_NAME

[English](README.md) | [中文](README_CN.md)

由 `gs k8s` 生成的 Kubernetes 部署脚手架。这里的一切都是**可改的起点**,
不是运行时依赖——请按你的集群调整。

## 目录结构

```
Dockerfile                 # 多阶段构建 -> distroless/static:nonroot
conf/app-k8s.properties    # k8s profile:JSON 日志到 stdout、drain 窗口
deploy/k8s/
  base/                    # Deployment、Service、HPA、ServiceMonitor
  overlays/dev             # 1 副本、dev- 前缀、:dev 镜像
  overlays/prod            # 3 副本、更紧的资源
```

## 构建与部署

```sh
docker build -t GS_IMAGE:latest .
kubectl apply -k deploy/k8s/overlays/dev     # 或 overlays/prod
```

无集群时试运行:

```sh
kubectl kustomize deploy/k8s/base            # 渲染 manifest
kubectl apply -k deploy/k8s/overlays/dev --dry-run=client
```

## 健康探针

探针全部命中 actuator **管理端口(GS_MGMT_PORT)**:

- `startupProbe` → `/startup` —— 应用启动完成后才通过,慢启动不会被 liveness 误杀。
- `livenessProbe` → `/health` —— 只看进程存活;依赖降级不会触发重启。
- `readinessProbe` → `/readiness` —— 优雅停机期间返回 503,先把 Pod 从 Service
  endpoints 摘掉再停 server。

## 优雅停机

`terminationGracePeriodSeconds`(30s)与容器 `preStop` sleep(5s)和
`conf/app-k8s.properties` 里的 `app.shutdown.timeout` / `app.shutdown.pre-stop-delay`
对齐。滚动更新无损排空在途请求。

## 指标

`servicemonitor.yaml` 抓取管理端口的 `/metrics`,需要 Prometheus Operator。
只有应用接了 **starter-otel** 才会暴露 `/metrics`,否则请删除该 ServiceMonitor。

## Pod 元数据

Deployment 通过 Downward API 注入 Pod 字段(`GS_POD_NAME`、`GS_POD_NAMESPACE`、
`GS_POD_IP`、`GS_NODE_NAME`、`GS_POD_SERVICE_ACCOUNT`),并把 labels/annotations
挂载到 `/etc/podinfo`。在应用里用 `go-spring.org/stdlib/podinfo` 读取:

```go
gs.Object(&podinfo.PodInfo{})
```
