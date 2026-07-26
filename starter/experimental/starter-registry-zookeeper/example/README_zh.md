# starter-registry-zookeeper Example

演示 starter-registry-zookeeper 的 ZooKeeper 服务注册与发现。

## 功能验证

- **服务注册**：启动时将服务注册到 ZooKeeper
- **服务发现**：通过 ZK API 查询已注册的 znode
- **自动注销**：进程退出时自动从 ZK 注销

> 需要 ZooKeeper 服务运行。`check.sh` 通过 docker compose 启动 ZooKeeper。

## 手动验证

```bash
cd starter-registry-zookeeper/example
go run . -manual
```

预期输出：
```
instances found: ...
```

需要先启动 ZooKeeper：
```bash
# 启动 ZooKeeper
docker compose up -d

# 运行示例
go run . -manual
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 ZooKeeper，运行示例并验证注册，退出码 0 表示通过。
