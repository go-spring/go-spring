# starter-config-k8s 示例

演示 starter-config-k8s 的 Kubernetes ConfigMap 配置加载。

## 功能验证

- **ConfigMap 加载**：从 K8s ConfigMap 读取配置
- **热更新**：监听 ConfigMap 变化并动态刷新

> 注意：读取并监听真实 ConfigMap 需要 Kubernetes 集群。无集群时示例仍能启动并
> 自动退出（import 是 `optional:`）。

## 手动验证

```bash
cd starter-config-k8s/example
go run . -manual
```

需要在 K8s 集群中运行（见 `deploy/`）。本地直接运行会打印提示并正常退出。

服务保持运行，`Ctrl+C` 退出。

另开一个终端编辑 ConfigMap，即可看到字段变化打印：

```bash
kubectl edit configmap app-config      # 或：kubectl patch configmap app-config ...
# 打印：demo.message: "..." -> "manual-1"
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 启动示例；无集群时跳过读取、字段取默认值并自动退出，退出码 0 表示通过。