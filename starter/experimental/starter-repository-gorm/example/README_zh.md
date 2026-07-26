# starter-repository-gorm Example

演示 starter-repository-gorm 的通用 Repository 模式。

## 功能验证

- **CRUD 操作**：Create/Read/Update/Delete 全流程
- **分页查询**：支持分页参数
- **复合条件**：多字段组合查询
- **审计字段**：自动填充创建人/修改人

## 手动验证

```bash
cd starter-repository-gorm/example
go run . -manual
```

程序保持运行，等待 Ctrl+C 退出。不带 -manual 时 runTest() 会自动执行并退出。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。