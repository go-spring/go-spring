# starter-swagger Example

演示 starter-swagger 的 Swagger UI 集成。

## 功能验证

- **Swagger UI**：提供交互式 API 文档界面
- **OpenAPI Spec**：自动生成 OpenAPI 规范 JSON
- **API 测试**：通过 UI 直接测试 API 端点

## 手动验证

```bash
cd starter-swagger/example
go run .
```

预期输出（断言通过后程序自动退出）：
```
Swagger UI accessible
OpenAPI spec returned
```

也可以手动验证：
```bash
# 终端1：启动服务
go run .

# 终端2
curl http://127.0.0.1:9090/swagger/index.html
# -> Swagger UI HTML

curl http://127.0.0.1:9090/swagger/doc.json
# -> OpenAPI spec JSON
```

浏览器打开 `http://127.0.0.1:9090/swagger/index.html` 查看交互式文档。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。