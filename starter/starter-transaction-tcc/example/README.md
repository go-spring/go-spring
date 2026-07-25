# starter-transaction-tcc Example

演示 starter-transaction-tcc 的 TCC 分布式事务（Try-Confirm-Cancel 模式）。

## 功能验证

- **提交路径**：Try → Confirm，两个服务数据一致
- **回滚路径**：Try → Cancel，数据恢复到事务前状态
- **事务协调**：Coordinator 管理 TCC 各阶段

## 手动验证

```bash
cd starter-transaction-tcc/example
go run .
```

预期输出（断言通过后程序自动退出）：
```
commit path: OK
rollback path: OK
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 运行示例并等待其自测完成，退出码 0 表示通过。