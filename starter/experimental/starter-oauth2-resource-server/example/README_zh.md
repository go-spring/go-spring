# starter-oauth2-resource-server 示例

演示一个用 JWT Bearer 认证保护端点的 OAuth2 资源服务器。校验使用共享
HMAC 密钥，因此不需要外部身份 provider——示例自己签发 token。

## 功能特性

- **Token 认证**：不带 Bearer Token 的请求返回 401
- **无效 Token 拒绝**：伪造 Token（密钥错误）返回 401
- **身份传播**：`/me` 回显认证后的主体
- **权限校验**：`/orders` 要求 `orders:read` scope，否则 403

## 手动验证

终端 1，启动服务：

```bash
cd starter/experimental/starter-oauth2-resource-server/example
go run . -manual
```

终端 2，执行验证命令：

```bash
# 无 Token -> 401
curl -i http://127.0.0.1:9090/me

# 带 Token -> 200
curl -i -H 'Authorization: Bearer <token>' http://127.0.0.1:9090/me
```

按 Ctrl+C 停止服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待自检完成，退出码 0 即通过。
