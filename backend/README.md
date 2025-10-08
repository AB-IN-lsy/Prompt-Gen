# Backend Development Guide

## Environment Secrets

The backend never hardcodes infrastructure credentials. Provide them at runtime via environment variables or your preferred secret manager.

Required variables:

| Variable | Description |
| --- | --- |
| `NACOS_ENDPOINT` | e.g. `8.140.xx.xx:8848` |
| `NACOS_USERNAME` | Nacos account name |
| `NACOS_PASSWORD` | Nacos account password |
| `NACOS_GROUP` | Optional, defaults to `DEFAULT_GROUP` |
| `NACOS_NAMESPACE` | Optional, defaults to `public` |
| `MYSQL_CONFIG_DATA_ID` | Optional override for config data ID |
| `MYSQL_CONFIG_GROUP` | Optional override for config group |
| `MYSQL_TEST_HOST` | Integration test host (only when running with `-tags integration`) |
| `MYSQL_TEST_USER` | Integration test username |
| `MYSQL_TEST_PASS` | Integration test password |
| `MYSQL_TEST_DB` | Integration test database name |
| `MYSQL_TEST_PARAMS` | Optional MySQL DSN query string |
| `NACOS_TEST_DATA_ID` | Integration test data ID |
| `NACOS_TEST_GROUP` | Integration test group |

Store real values in `.env.local` (ignored by git) or system secret storage. The backend automatically attempts to load `.env.local` and `.env` on startup via `config.LoadEnvFiles()`, so placing the file alongside the executable is sufficient.

To generate a fresh template:

```powershell
Copy-Item ..\..\.env.example ..\..\.env.local -Force
```

## Running Tests

- Unit tests (no external dependencies, located in `backend/tests/unit`):

  ```powershell
  cd backend
  go test ./...
  ```

- Integration tests (require reachable Nacos / MySQL and the env vars above, located in `backend/tests/integration`):

  ```powershell
  cd backend
  $env:INTEGRATION = "1"
  go test -tags integration ./...
  ```

Unset `INTEGRATION` afterwards to avoid accidental real connections.
