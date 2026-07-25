# starter-registry-nacos Example

演示 starter-registry-nacos 的 Nacos 服务注册与发现。

## 功能验证

- **服务注册**：启动时将服务注册到 Nacos
- **服务发现**：通过 Nacos Open API 查询已注册的服务实例
- **自动注销**：进程退出时自动从 Nacos 注销

> 需要 Nacos 服务运行。`check.sh` 通过 docker compose 启动 Nacos。

## 手动验证

```bash
cd starter-registry-nacos/example
go run .
```

预期输出：
```
instances found: ...
```

需要先启动 Nacos：
```bash
# 启动 Nacos
docker compose up -d

# 运行示例
go run .

# 查看注册的服务
curl 'http://127.0.0.1:8848/nacos/v1/ns/instance/list?serviceName=go-spring'
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Nacos，运行示例并验证注册，退出码 0 表示通过。