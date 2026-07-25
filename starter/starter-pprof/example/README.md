# starter-pprof Example

演示 starter-pprof 的 pprof 端点 Token 鉴权。

## 功能验证

- **Token 鉴权**：无 Token 的请求返回 401 Unauthorized
- **Bearer Token**：携带 `Authorization: Bearer s3cr3t` 的请求正常访问
- **pprof 端点**：`/debug/pprof/`、`/debug/pprof/heap`、`/debug/pprof/cmdline`

## 手动验证

```bash
cd starter-pprof/example
go run .
```

预期输出：
```
Rejected without token: /debug/pprof/ 401
Rejected without token: /debug/pprof/heap 401
Rejected without token: /debug/pprof/cmdline 401
Response from server: /debug/pprof/ 200
Response from server: /debug/pprof/heap 200
Response from server: /debug/pprof/cmdline 200
```

也可以手动 curl 验证：
```bash
# 终端1：启动服务
go run .

# 终端2：测试各端点
# 无 Token -> 401
curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:9981/debug/pprof/
# -> 401

# 有 Token -> 200
curl -s -o /dev/null -w '%{http_code}' -H 'Authorization: Bearer s3cr3t' \
  http://127.0.0.1:9981/debug/pprof/
# -> 200

curl -s -o /dev/null -w '%{http_code}' \
  'http://127.0.0.1:9981/debug/pprof/heap?token=s3cr3t'
# -> 200
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。