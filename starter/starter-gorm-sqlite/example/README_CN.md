# starter-gorm-sqlite 示例

一次面向内存 SQLite 的自包含往返：版本查询、自动建表、增/查/事务更新。
成功后自退出（`SIGTERM`）；任何失败以非零码退出。无需 docker。

## 运行

```bash
go run .
./check.sh   # 冒烟测试
```

手动验证模式会让服务保持运行：

```bash
go run . -manual
```
