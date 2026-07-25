# starter-admin-ui Example

演示 starter-admin-ui 的管理界面。

## 功能验证

- **管理界面**：提供 Web UI 查看应用状态和配置

## 手动验证

```bash
cd starter-admin-ui/example
go run .
```

浏览器打开 `http://127.0.0.1:9090` 查看管理界面。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。