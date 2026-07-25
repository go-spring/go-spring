# starter-oauth2-client Example

Demonstrates OAuth2 client for token acquisition and bearer authentication.

## Features

- **Token acquisition**: Gets an access token from the authorization server
- **Bearer authentication**: Attaches the token to the Authorization header of downstream requests
- **Auto-refresh**: Refreshes the token on expiry

## Manual Testing

```bash
cd starter-oauth2-client/example
go run . -manual
```

The service keeps running. Press Ctrl+C to exit. Without `-manual`, `runTest()` executes automatically and exits.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
