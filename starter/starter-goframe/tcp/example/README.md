# starter-goframe/tcp Example

演示 starter-goframe/tcp 的 TCP 行回声服务。

## 功能验证

- **TCP 回声**：发送 `ping\n`，接收 `ping\n`（行回声）

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-goframe/tcp/example
go run . -manual
```

终端 2，执行验证命令：
```bash
echo "hello" | nc 127.0.0.1 8003
# -> hello
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。