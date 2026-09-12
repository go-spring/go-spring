# starter-config-apollo 示例

自包含冷加载：示例启动一个 mock Apollo config service（meta + configfiles
端点），导入 starter，断言远程属性冷加载进 Dync 字段。无需 docker，也无需
真实 Apollo 栈。

## 运行

```bash
go run .
./check.sh   # 冒烟测试
```
