# starter-lock-k8s Example

演示 starter-lock-k8s 的 Kubernetes Lease 分布式锁。

## 功能验证

- **Leader 选举**：通过 K8s Lease 资源竞争 leadership
- **锁获取/释放**：获取锁后执行业务逻辑，释放锁

> 注意：此示例需要在 Kubernetes 集群中运行。`check.sh` 在无集群环境下会跳过。

## 手动验证

需要在 K8s 集群中运行。

终端 1，启动服务并保持运行：
```bash
cd starter-lock-k8s/example
go run . -manual
```

本地直接运行会打印提示并正常退出。验证完成后 `Ctrl+C` 退出。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例，无 K8s 集群时自动跳过。