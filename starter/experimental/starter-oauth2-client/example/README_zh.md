# starter-oauth2-client Example

演示 starter-oauth2-client 的 OAuth2 客户端。

## 功能验证

- **Token 获取**：自动从授权服务器获取 access token
- **Bearer 认证**：将 token 附加到下游请求的 Authorization 头
- **自动刷新**：token 过期后自动刷新

## 手动验证

```bash
cd starter-oauth2-client/example
go run . -manual
```

程序保持运行，等待 Ctrl+C 退出。不带 -manual 时 runTest() 会自动执行并退出。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。