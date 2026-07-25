# starter-migration-gorm Example

演示 starter-migration-gorm 的数据库迁移。

## 功能验证

- **启动迁移**：应用启动时自动执行 pending 的迁移脚本
- **幂等性**：第二次运行不会重复执行已完成的迁移
- **校验和保护**：篡改已执行的迁移脚本会导致启动失败（checksum 不匹配）

## 手动验证

```bash
cd starter-migration-gorm/example
go run .
```

预期输出（断言通过后程序自动退出）：
```
migrations applied on startup
second run idempotent
checksum mismatch detected
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。