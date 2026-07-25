# starter-config-vault Example

演示 starter-config-vault 的 Vault 加密配置管理。

## 功能验证

- **加密配置**：从 Vault 读取加密的配置项
- **配置解密**：通过 AES 密钥解密配置值
- **配置热更新**：发布新的加密值后应用实时感知

> 需要 Vault 服务运行。`check.sh` 通过 docker compose 启动 Vault。

## 手动验证

```bash
cd starter-config-vault/example
go run .
```

预期输出：
```
initial value
updated value
```

需要先启动 Vault：
```bash
# 启动 Vault
docker compose up -d

# 运行示例
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Vault，运行示例并验证配置刷新，退出码 0 表示通过。