# starter-bigcache Example

演示 starter-bigcache 的进程内热缓存。

## 功能验证

- **SET/GET**：写入缓存项并读取验证
- **TTL 过期**：写入短 TTL 项，等待过期后验证返回不存在
- **HTTP 端点**：通过 `/get` 端点读取缓存值

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-bigcache/example
go run . -manual
```

终端 2，执行验证命令：
```bash
curl http://127.0.0.1:9090/get
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。
