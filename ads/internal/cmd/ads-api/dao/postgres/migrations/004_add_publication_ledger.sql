CREATE TABLE ad_publications (
    id           BIGSERIAL PRIMARY KEY,
    user_id      UUID NOT NULL,
    published_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ad_publications_user_time
    ON ad_publications (user_id, published_at);