# starter-go-zero/rest Example

演示 starter-go-zero/rest 的 HTTP 服务路由。

## 功能验证

- **路由**：`/greet` 查询参数 + JSON 响应

## 手动验证

```bash
cd starter-go-zero/rest/example
go run .
```

预期输出：
```
Response from server: {"message":"Hi, world"}
```

也可以手动 curl 验证：
```bash
# 终端1：启动服务
go run .

# 终端2
curl 'http://127.0.0.1:8888/greet?name=world'
# -> {"message":"Hi, world"}
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。