# starter-memcached Example

演示 starter-memcached 的 Memcached 客户端。

## 功能验证

- **String SET/GET**：写入和读取字符串键值
- **多 Driver 支持**：可切换不同 Memcached driver

> 需要 Memcached 服务运行。`check.sh` 通过 docker compose 启动 Memcached。

## 手动验证

```bash
cd starter-memcached/example
go run .
```

预期输出：
```
SET foo bar: OK
GET foo: bar
```

需要先启动 Memcached：
```bash
# 启动 Memcached
docker compose up -d

# 运行示例
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Memcached，运行示例并验证操作，退出码 0 表示通过。