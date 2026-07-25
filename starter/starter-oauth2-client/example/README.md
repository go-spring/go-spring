# starter-oauth2-client Example

Demonstrates OAuth2 client for starter-oauth2-client.

## Features

- **Token acquisition**: Automatically obtains access token from the authorization server
- **Bearer authentication**: Attaches token to the Authorization header of downstream requests
- **Auto-refresh**: Automatically refreshes token upon expiry

## Manual Testing

```bash
cd starter-oauth2-client/example
go run . -manual
```

The program keeps running, press Ctrl+C to exit. Without -manual, runTest() executes automatically and exits.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
