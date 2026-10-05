# starter-config-vault 示例

演示 starter-config-vault 的 Vault 加密配置管理。

## 功能验证

- **加密配置**：从 Vault 读取加密的配置项
- **配置解密**：通过 AES 密钥解密配置值
- **配置热更新**：发布新的加密值后应用实时感知

> 需要 Vault 服务运行。`check.sh` 通过 docker compose 启动 Vault。

## 手动验证

```bash
cd starter-config-vault/example
go run . -manual
```

需要先启动 Vault，并把 token 与 AES 解密密钥放进环境（见 `check.sh`）：
```bash
# 启动 Vault
docker compose up -d

export VAULT_TOKEN=root
export GS_CONFIG_DECRYPT_AES_KEY=MTIzNDU2Nzg5MDEyMzQ1Ng==

# 运行示例（manual 模式，保持运行）
go run . -manual
```

服务保持运行，`Ctrl+C` 退出服务。另开一个终端发布新文档，即可看到两个字段的
变化打印：

```bash
curl -fsS -X POST -H "X-Vault-Token: $VAULT_TOKEN" \
  -d '{"data":{"application.properties":"demo.message=manual-1\n"}}' \
  http://127.0.0.1:8200/v1/secret/data/gs-config-demo
# 打印：demo.message: "..." -> "manual-1"
```

不带 `-manual` 时，示例会写入一份新的（部分加密的）文档到 Vault、等待两个
绑定字段热更新，打印 `hot-reload observed: hello-<hhmmss>` 与
`decrypted password: topsecret` 后退出：

```bash
go run .
```

## 冒烟测试

```bash
./check.sh
```

`check.sh` 通过 docker compose 启动 Vault，运行示例并验证配置刷新，退出码 0 表示通过。