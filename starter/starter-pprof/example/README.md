# starter-pprof Example

演示 starter-pprof 的 pprof 端点 Token 鉴权。

## 功能验证

- **Token 鉴权**：无 Token 的请求返回 401 Unauthorized
- **Bearer Token**：携带 `Authorization: Bearer s3cr3t` 的请求正常访问
- **pprof 端点**：`/debug/pprof/`、`/debug/pprof/heap`、`/debug/pprof/cmdline`

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-pprof/example
go run . -manual
```

终端 2，执行验证命令：
```bash
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

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。