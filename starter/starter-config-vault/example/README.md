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

Vault must be running first, and the token plus the AES decrypt key must be in
the environment (see `check.sh`):
```bash
# Start Vault
docker compose up -d

export VAULT_TOKEN=root
export GS_CONFIG_DECRYPT_AES_KEY=MTIzNDU2Nzg5MDEyMzQ1Ng==

# Run example (manual mode, keeps running)
go run . -manual
```

The service keeps running. Press `Ctrl+C` to stop.

In another terminal, publish a new document and watch both fields print:

```bash
curl -fsS -X POST -H "X-Vault-Token: $VAULT_TOKEN" \
  -d '{"data":{"application.properties":"demo.message=manual-1\n"}}' \
  http://127.0.0.1:8200/v1/secret/data/gs-config-demo
# prints: demo.message: "..." -> "manual-1"
```

Without `-manual` the example writes a new (partly encrypted) document to
Vault, waits for both bound fields to hot-reload, prints
`hot-reload observed: hello-<hhmmss>` and `decrypted password: topsecret`, and
exits:

```bash
go run .
```

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Vault via docker compose, runs the example and verifies config refresh, exit code 0 means pass.