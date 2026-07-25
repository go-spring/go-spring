# starter-mail Example

演示 starter-mail 的邮件发送。

## 功能验证

- **邮件发送**：通过 SMTP 发送邮件
- **连接验证**：验证 SMTP 服务器连通性

> 需要 SMTP 服务（Mailpit/MailHog）运行。`check.sh` 通过 docker compose 启动 Mailpit。

## 手动验证

```bash
cd starter-mail/example
go run .
```

预期输出：
```
mail sent successfully
```

需要先启动 Mailpit：
```bash
# 启动 Mailpit
docker compose up -d

# 运行示例
go run .

# 查看发送的邮件
open http://127.0.0.1:8025
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Mailpit，运行示例并验证邮件发送，退出码 0 表示通过。