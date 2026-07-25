# starter-goframe/tcp Example

演示 starter-goframe/tcp 的 TCP 行回声服务。

## 功能验证

- **TCP 回声**：发送 `ping\n`，接收 `ping\n`（行回声）

## 手动验证

```bash
cd starter-goframe/tcp/example
go run .
```

预期输出：
```
Response from server: ping
```

也可以手动 nc 验证：
```bash
# 终端1：启动服务
go run .

# 终端2
echo "hello" | nc 127.0.0.1 8003
# -> hello
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。