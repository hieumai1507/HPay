[solution-architecture →](solution-architecture.md) · m5-api-support

# M5 API Support — Implementation Plan

## 1. Scope

Add a supporting API that lists the businesses of which the current user is a member (serving the personal wallet <> business wallet switching screen on Mobile).

## 2. Task Breakdown

| # | Task | Main files | Size | Depends |
| --- | --- | --- | --- | --- |
| T1 | Update `openapi.yaml` to add the `get` method for `/v1/org` | `api/openapi.yaml` | S | — |
| T2 | Add the SQL query `ListUserOrganizations` in `org.sql` and run `make sqlc` | `internal/store/queries/org.sql` | S | — |
| T3 | Update `internal/org/service.go` to add the function that lists businesses | `internal/org/service.go` | S | T2 |
| T4 | Add the `ListMyOrgs` handler in `org_handler.go` and map the route in `router.go` | `internal/api/org_handler.go`, `internal/api/router.go` | S | T3 |
| T5 | Write integration tests for the new API | `test/integration/org_test.go` (or `qr_test.go`) | M | T4 |

## 3. API Specification

Details of the added route:

```text
GET /v1/org
Headers: Authorization: Bearer <JWT>
Response: 200 OK
{
  "organizations": [
    {
      "org_id": "uuid",
      "name": "string",
      "role": "admin|maker|checker",
      "checker_level": 0,
      "wallet_id": "uuid",
      "balance": 15000000
    }
  ]
}
```

## 4. Verification

After implementation, run the backend test suite:

```text
make lint
go test -v -run TestIntegration_ListUserOrgs ./test/integration/...
```
