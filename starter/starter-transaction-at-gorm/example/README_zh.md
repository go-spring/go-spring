# starter-transaction-at-gorm Example

演示 starter-transaction-at-gorm 的 AT 分布式事务（Seata AT 模式）。

## 功能验证

- **提交路径**：正常事务提交，两个数据库一致
- **回滚路径**：事务失败后自动回滚，数据恢复到事务前状态
- **写写隔离**：并发写操作的事务隔离保证

## 手动验证

```bash
cd starter-transaction-at-gorm/example
go run . -manual
```

程序保持运行，等待 Ctrl+C 退出。不带 -manual 时 runTest() 会自动执行并退出。

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。