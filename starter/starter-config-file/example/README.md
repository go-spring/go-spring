# starter-config-file Example

演示 starter-config-file 的本地文件配置热更新。

## 功能验证

- **配置加载**：从 `conf/app.properties` 读取初始值
- **文件监听**：通过 `Dync[T]` 监听文件变化，自动热更新
- **动态值验证**：修改文件后配置值实时变化

## 手动验证

```bash
cd starter-config-file/example
go run .
```

预期输出：
```
initial
updated
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。