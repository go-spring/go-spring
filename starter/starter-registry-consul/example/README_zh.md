# starter-registry-consul Example

演示 starter-registry-consul 的 Consul 服务注册与发现。

## 功能验证

- **服务注册**：启动时将服务注册到 Consul catalog
- **服务发现**：通过 Consul API 查询已注册的服务实例
- **健康检查**：Consul 定期检查服务健康状态

> 需要 Consul 服务运行。`check.sh` 通过 docker compose 启动 Consul。

## 手动验证

```bash
cd starter-registry-consul/example
go run . -manual
```

预期输出：
```
instances found: ...
```

需要先启动 Consul：
```bash
# 启动 Consul
docker compose up -d

# 运行示例
go run . -manual

# 查看注册的服务
curl http://127.0.0.1:8500/v1/catalog/service/go-spring
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Consul，运行示例并验证注册，退出码 0 表示通过。
