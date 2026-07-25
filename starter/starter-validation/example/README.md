# starter-validation Example

Demonstrates data validation and i18n internationalization using starter-validation.

## Features

- **Config-binding validation**: Validates struct fields after config binding (email format, port minimum), outputs localized error messages on failure
- **Web path validation**: HTTP handler decodes request body and validates it, returns structured 400 on failure
- **i18n**: The same `ValidationErrors` rendered in English or Chinese, messages loaded from `messages_*.yaml`

## Manual Testing

```bash
cd starter-validation/example
go run .
```

Expected output (English):
```
== config-binding path ==
 - admin must be a valid email address
 - port must be at least 1024

== web path (en) ==
 status=400 body={"errors":["email must be a valid email address","age must be at least 18"]}

== web path (zh) ==
 status=400 body={"errors":["email must be a valid email address","age must be at least 18"]}

== web path (valid) ==
 status=200 body=ok: a@b.com
```

## Smoke Test

```bash
./check.sh
```

check.sh runs the example and waits for self-test to complete, exit code 0 means pass.
