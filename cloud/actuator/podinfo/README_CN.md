# podinfo

[English](README.md) | [中文](README_CN.md)

`podinfo` 把 Kubernetes Pod 元数据（name、namespace、UID、IP、node、host IP、
service account、labels）暴露给应用，**零第三方依赖**。

它**不**访问 Kubernetes API server：不 import client-go、不 informer、不 watch。
所有字段都来自配置或挂载文件，经由
[Downward API](https://kubernetes.io/zh-cn/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/)
注入：Deployment 把 Pod 字段以环境变量注入，并把 labels/annotations 以文件形式挂载。
Go-Spring 配置层会把 `GS_` 前缀的环境变量映射进属性树（`GS_POD_NAME` → `pod.name`），
因此 `PodInfo` 的字段直接从配置绑定。

## 安装

```
go get go-spring.org/cloud
```

## 环境变量约定

`gs k8s` 脚手架生成的 Deployment 会通过 Downward API 接好这些变量。若手写 manifest，
请对齐以下命名：

| 环境变量                    | 属性                   | Downward API 来源           |
|----------------------------|-----------------------|-----------------------------|
| `GS_POD_NAME`              | `pod.name`            | `metadata.name`             |
| `GS_POD_NAMESPACE`         | `pod.namespace`       | `metadata.namespace`        |
| `GS_POD_UID`               | `pod.uid`             | `metadata.uid`              |
| `GS_POD_IP`                | `pod.ip`              | `status.podIP`              |
| `GS_NODE_NAME`             | `node.name`           | `spec.nodeName`             |
| `GS_NODE_IP`               | `node.ip`             | `status.hostIP`             |
| `GS_POD_SERVICE_ACCOUNT`   | `pod.service.account` | `spec.serviceAccountName`   |
| `pod.labels.path`（配置）  | `pod.labels.path`     | labels 卷的挂载路径          |

> 注意：`pod.labels.path` 在 `k8s` 配置 profile 里设置（不是环境变量），因为
> `GS_` env → 属性的映射无法产出连字符或任意路径。labels 以文件形式挂载
> （如 `/etc/podinfo/labels`）。

## 用法

`PodInfo` 带 `value` 标签，但不 import IoC 容器，因此无法自注册。注册成 bean
（init 里一行），哪里要用 Pod 事实就 autowire 指针：

```go
func init() { gs.Object(&podinfo.PodInfo{}) }

type MyService struct {
    Pod *podinfo.PodInfo `autowire:""`
}

func (s *MyService) Describe() {
    fmt.Println(s.Pod.Name, s.Pod.Namespace, s.Pod.IP)
    labels, _ := s.Pod.Labels() // 解析挂载的 labels 文件
    fmt.Println(labels["app"])
}
```

bean 由谁提供：**应用自己**——即上面的 `gs.Object(&podinfo.PodInfo{})` 一行。
框架不提供，这一行需要应用自己来写。

在 Kubernetes 之外（没有 Downward API 变量）时，每个字段为空，`Labels()` 返回空 map
——应用照常 wire、照常运行。`value` 标签默认空值而非校验失败，也是这个原因：同一
段代码在笔记本和集群里都能跑。
