# starter-config-k8s

[English](README.md) | [中文](README_CN.md)

`starter-config-k8s` **直接通过 API Server 读取 Kubernetes ConfigMap 或 Secret**,作为
可热更新的配置源。它与 [starter-config-file](../starter-config-file) 互补:file starter 监听
挂载卷,受 kubelet 投影延迟影响(Secret 轮转约 1 分钟);本 starter 用 client-go informer 直接
watch 对象,`kubectl edit configmap` 数秒内即可传播到绑定的 `gs.Dync` 字段,且可跨
ServiceAccount 有权读取的任意命名空间。

空导入本 starter 会注册一个 `k8s` 配置 provider,通过 `spring.app.imports` 消费。这是一个
**纯 provider** starter:没有可注入的配置 bean,连接参数从 source 串解析。

| Starter | 机制 | 依赖 | RBAC | 适用 |
| --- | --- | --- | --- | --- |
| `starter-config-file` | 监听挂载卷(`..data` 软链切换) | 无 | 无 | 零权限;可容忍 kubelet 投影延迟。 |
| `starter-config-k8s` | 对 ConfigMap/Secret 的 client-go informer | client-go | `get/list/watch` | 即时传播;可跨命名空间。 |

## 安装

```bash
go get go-spring.org/starter-config-k8s
```

## 快速开始

### 1. 导入包

```go
import _ "go-spring.org/starter-config-k8s"
```

### 2. 从 ConfigMap/Secret 导入配置

source 形式:`<kind>/<name>[?namespace=..&key=..&format=..&kubeconfig=..]`。

```properties
spring.app.imports=k8s:configmap/app-config?namespace=default&key=application.yaml
```

- `secret/<name>` 改为读取 Secret(其 `data` 已完成 base64 解码)。
- 加 `optional:`(`optional:k8s:configmap/...`)可在对象或集群缺失时仍正常启动——跳过读取,
  绑定字段回退到默认值。
- 集群外运行时追加 `&kubeconfig=/home/me/.kube/config`。

### 3. 绑定可热更新字段

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

`kubectl edit configmap app-config`(修改 `demo.message`)数秒内即更新绑定字段,无需重启。

## source 参数

| 部分 | 默认 | 说明 |
| --- | --- | --- |
| `<kind>` | — | `configmap` 或 `secret`(必填)。 |
| `<name>` | — | 对象名(必填)。 |
| `namespace` | `default` | 对象命名空间。 |
| `key` | (全部) | 设置后只读取该条 `data`。 |
| `format` | (按扩展名) | 为无可识别扩展名的条目强制指定解析器(`yaml`/`properties`/`toml`/`json`)。 |
| `kubeconfig` | (空) | kubeconfig 路径;留空使用集群内认证。 |

每条 `data` 按其 key 的扩展名解析为配置文档(`application.yaml` → YAML)并展平为属性,与
file starter 的目录语义一致;无可识别扩展名且未强制 `format` 的条目会被跳过(除非通过 `key`
显式选中)。

## 工作原理

- `loadK8sConfig` 启动时读取一次对象,将其 `data` 展平为属性,并安装一个限定命名空间与名称的
  informer。
- 对象的每次 add/update/delete 触发一次全局属性 refresh,重跑 provider 并把新值传播到绑定的
  `gs.Dync` 字段。refresh 通过一个注入框架 `PropertiesRefresher` 的 `gs.Rooter` 桥接 bean
  接线(稳定 bean 名以避开 `__default__` Rooter 冲突)。
- 桥接 bean 的析构函数在关停时停止所有 informer。

## RBAC

ServiceAccount 需在其命名空间内对目标 `configmaps`/`secrets` 拥有 `get/list/watch`
(`get` 用于首次读取,`list/watch` 用于 informer)。见
[example/deploy/rbac.yaml](example/deploy/rbac.yaml)。

## 集群内验证

单元测试用 client-go fake clientset 覆盖了解析/读取/key 过滤/optional 缺失以及 informer 驱动的
refresh。端到端热更新需要真实集群:

```bash
kubectl apply -f example/deploy/rbac.yaml
kubectl apply -f example/deploy/configmap.yaml
# 为 example/ 构建并推送镜像,再 apply example/deploy/deployment.yaml,然后:
kubectl logs deploy/config-k8s-example      # 打印来自 ConfigMap 的 demo.message
kubectl edit configmap app-config           # 修改 demo.message;字段热更新
```
