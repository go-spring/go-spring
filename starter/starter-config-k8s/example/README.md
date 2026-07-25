# starter-config-k8s Example

演示 starter-config-k8s 的 Kubernetes ConfigMap 配置加载。

## 功能验证

- **ConfigMap 加载**：从 K8s ConfigMap 读取配置
- **热更新**：监听 ConfigMap 变化并动态刷新

> 注意：此示例需要在 Kubernetes 集群中运行。`check.sh` 在无集群环境下会跳过。

## 手动验证

```bash
cd starter-config-k8s/example
go run .
```

需要在 K8s 集群中运行，本地直接运行会跳过实际功能。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例，无 K8s 集群时自动跳过。