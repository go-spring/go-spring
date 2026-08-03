# starter-registry-etcd Example

演示 starter-registry-etcd 的 etcd 服务注册与发现。

## 功能验证

- **服务注册**：启动时将服务注册到 etcd
- **服务发现**：通过 etcd API 查询已注册的服务 key
- **自动注销**：进程退出时自动从 etcd 注销

> 需要 etcd 服务运行。`check.sh` 通过 docker compose 启动 etcd。

## 手动验证

```bash
cd starter-registry-etcd/example
go run . -manual
```

预期输出：
```
instances found: ...
```

需要先启动 etcd：
```bash
# 启动 etcd
docker compose up -d

# 运行示例
go run . -manual

# 查看注册的 key
etcdctl get --prefix /go-spring/
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 etcd，运行示例并验证注册，退出码 0 表示通过。
