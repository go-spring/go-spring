# starter-security-jwt Example

演示 starter-security-jwt 的 JWT 认证。

## 功能验证

- **JWT 认证**：通过 JWT token 验证请求身份
- **无 Token 拒绝**：缺少 token 的请求返回 401
- **有效 Token 放行**：携带有效 token 的请求到达业务 handler

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-security-jwt/example
go run . -manual
```

终端 2，执行验证命令：
```bash
# 无 Token -> 401
curl -i http://127.0.0.1:9090/me
# -> HTTP/1.1 401 Unauthorized

# 有 Token -> 200
curl -i -H 'Authorization: Bearer <token>' http://127.0.0.1:9090/me
# -> HTTP/1.1 200 OK
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。
