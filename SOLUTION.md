# Solution

The changes are confined to the Ads API (and shared code where applicable). The Users service remains unchanged. The existing HTTP API is preserved where possible, with compatibility maintained for existing clients.

## 1. Fix `GetAdByID`

Corrected the use-case implementation so it retrieves an ad by its ID and propagates the appropriate errors. Added unit and end-to-end coverage for successful retrieval, invalid IDs, and missing ads.

## 2. Delete an ad

Added `DELETE /ads/{adID}` across the HTTP handler, use case, and DAO. Only the owner can delete an ad; the endpoint distinguishes missing authentication, forbidden access, and nonexistent ads. A successful deletion returns `204 No Content`.

## 3. Include the owner's username

`GET /ads/{adID}` enriches the ad with `user_name` from the provided Users service client. User information remains owned by the Users service rather than being duplicated in the Ads database. The read endpoint is the integration point; the Users service itself is unchanged.

## 4. Introduce decimal `price`

Exposed `price` in major currency units while retaining the legacy `price_cents` API field for compatibility. Create/import requests accept exactly one representation, validate cent-level precision, and normalize to integer cents internally. Integer storage avoids floating-point rounding in persisted monetary values. The old response field is documented as deprecated.

## 5. Make bulk imports atomic and retry-safe

`POST /ads/import` inserts all ads in one PostgreSQL transaction, rolling back the entire batch if any insertion fails. An optional `Idempotency-Key` is scoped to the authenticated user. The key, normalized request hash, and resulting ad IDs are recorded transactionally: identical retries return the original IDs; a different payload with the same key returns `409 Conflict`. Without a key, imports are atomic but repeated requests can still create duplicates. Normalization treats equivalent `price` and `price_cents` values consistently.

## 6. Enforce the publication quota

`POST /ads` allows at most five publications per user in a rolling 24-hour window. Limit and window are added to configuration. A per-user PostgreSQL transaction-level advisory lock serializes concurrent attempts; the quota check, ad insertion, and publication-ledger entry are committed together. Deleting an ad does not restore quota. Bulk imports are exempt.

## Verification

- `make test` runs unit tests with the Go race detector, starts a clean Docker Compose environment, runs E2E tests with the race detector, and tears down the environment.
- E2E coverage includes ownership, backward-compatible pricing, import idempotency, transaction rollback under a simulated database failure, concurrent quota enforcement, and rolling-window expiration.
- `make mount`, `make test-e2e`, and `make unmount` support running E2E tests repeatedly against a mounted environment.

## Trade-offs

- The API retains `price_cents` during migration to avoid breaking existing consumers; it can be removed in a future version after clients migrate.
- Idempotency records and publication history are retained for now. A production deployment should define retention and pruning policies, with care not to invalidate promised retry behavior.
- Username enrichment adds a Users service dependency to single-ad reads. For higher traffic, consider caching with an explicit freshness policy and observability around upstream latency and failures.
