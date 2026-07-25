# starter-mail Example

Demonstrates email sending with starter-mail.

## Features

- **Email Sending**: Send emails via SMTP
- **Connection Verification**: Verify SMTP server connectivity

> Requires SMTP service (Mailpit/MailHog) running. check.sh starts Mailpit via docker compose.

## Manual Testing

```bash
cd starter-mail/example
go run . -manual
```

Expected output:
```
mail sent successfully
```

Start Mailpit first:
```bash
# Start Mailpit
docker compose up -d

# Run example (manual mode, keep running)
go run . -manual

# View sent emails
open http://127.0.0.1:8025
```

The service keeps running. You can verify with corresponding CLI tools. Press Ctrl+C to stop.

## Smoke Test

```bash
./check.sh
```

check.sh starts Mailpit via docker compose, runs the example and verifies email sending, exit code 0 means pass.