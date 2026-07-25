# starter-oauth2-server Example

演示 starter-oauth2-server 的 OAuth2 授权服务器。

## 功能验证

- **授权端点**：`/oauth2/authorize` 处理授权请求
- **Token 端点**：`/oauth2/token` 签发 access token
- **Token 验证**：资源服务器通过 HMAC 验证 token
- **安全过滤器**：CORS + 认证 + 授权统一过滤器链

## 手动验证

```bash
cd starter-oauth2-server/example
go run .
```

预期输出（断言通过后程序自动退出）：
```
token issued
resource protected
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。