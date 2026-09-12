# starter-registry-k8s Example

演示 starter-registry-k8s 的 Kubernetes 服务发现。

## 功能验证

- **Service 发现**：通过 K8s API 发现集群中的 Service 实例
- **后端接线**：声明 K8s discovery 后端 bean（`k8s.main`）并经由它解析 Service

> 注意：此示例需要在 Kubernetes 集群中运行。`check.sh` 在无集群环境下会跳过。

## 手动验证

需要在 K8s 集群中运行。

终端 1，启动服务并保持运行：
```bash
cd starter-registry-k8s/example
go run . -manual
```

本地直接运行会打印提示并正常退出。验证完成后 `Ctrl+C` 退出。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例，无 K8s 集群时自动跳过。