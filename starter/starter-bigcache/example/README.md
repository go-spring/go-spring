# starter-bigcache Example

演示 starter-bigcache 的进程内热缓存。

## 功能验证

- **SET/GET**：写入缓存项并读取验证
- **TTL 过期**：写入短 TTL 项，等待过期后验证返回不存在
- **HTTP 端点**：通过 `/get` 端点读取缓存值

## 手动验证

```bash
cd starter-bigcache/example
go run .
```

预期输出：
```
Set key=foo value=bar
Got key=foo value=bar
Entry not found (expired as expected)
```

也可以手动 curl 验证：
```bash
# 终端1：启动服务
go run .

# 终端2
curl http://127.0.0.1:9090/get
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。