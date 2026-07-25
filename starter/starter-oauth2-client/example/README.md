# starter-oauth2-client Example

演示 starter-oauth2-client 的 OAuth2 客户端。

## 功能验证

- **Token 获取**：自动从授权服务器获取 access token
- **Bearer 认证**：将 token 附加到下游请求的 Authorization 头
- **自动刷新**：token 过期后自动刷新

## 手动验证

```bash
cd starter-oauth2-client/example
go run .
```

预期输出（断言通过后程序自动退出）：
```
token acquired
resource accessed
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。