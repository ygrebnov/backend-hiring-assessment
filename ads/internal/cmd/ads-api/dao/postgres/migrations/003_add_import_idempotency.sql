-- Store completed bulk imports to support safe retries.
-- Idempotency is scoped to the user, allowing different users
-- to reuse the same key independently.

CREATE TABLE ad_imports (
    user_id         UUID        NOT NULL,
    idempotency_key TEXT        NOT NULL,
    request_hash    TEXT        NOT NULL,
    ad_ids          BIGINT[]    NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, idempotency_key)
);