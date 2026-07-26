# starter-config-vault Example

Vault encrypted config management with starter-config-vault.

## Features

- **Encrypted config**: Read encrypted config items from Vault
- **Config decryption**: Decrypt config values via AES key
- **Config hot reload**: The app picks up changes in real time after publishing new encrypted values

> Requires a running Vault service. `check.sh` starts Vault via docker compose.

## Manual Testing

```bash
cd starter-config-vault/example
go run . -manual
```

Expected output:
```
initial value
updated value
```

Start Vault first:
```bash
# Start Vault
docker compose up -d

# Run example (manual mode, keeps running)
go run . -manual
```

The service keeps running. Press `Ctrl+C` to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Vault via docker compose, runs the example and verifies config refresh, exit code 0 means pass.