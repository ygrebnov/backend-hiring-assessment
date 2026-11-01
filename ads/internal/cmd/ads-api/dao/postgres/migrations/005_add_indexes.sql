-- Filter ads by owner.
CREATE INDEX idx_ads_user_id
    ON ads (user_id);

-- Filter ads by price range.
CREATE INDEX idx_ads_price_cents
    ON ads (price_cents);

-- Accelerate substring search by title on a sufficiently large table.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_ads_title_trgm
    ON ads USING GIN (title gin_trgm_ops);