# starter-casbin Example

演示 starter-casbin 的 RBAC 权限控制。

## 功能验证

- **策略文件加载**：从 `model.conf` 和 `policy.csv` 加载 RBAC 策略
- **权限判断**：通过 `/enforce` 端点判断 subject/object/action 是否允许
- **策略热更新**：通过 Watcher 监听策略文件变化

## 手动验证

终端 1，启动服务并保持运行：
```bash
cd starter-casbin/example
go run . -manual
```

终端 2，执行验证命令：
```bash
curl 'http://127.0.0.1:9090/enforce?sub=alice&obj=/data&act=write'
# -> allow

curl 'http://127.0.0.1:9090/enforce?sub=bob&obj=/data&act=write'
# -> deny
```

验证完成后 `Ctrl+C` 退出服务。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。
