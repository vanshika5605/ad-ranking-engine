-- Campaigns: advertisers and budget
CREATE TABLE IF NOT EXISTS campaigns (
    id              SERIAL PRIMARY KEY,
    advertiser_id   VARCHAR(64) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    status          VARCHAR(32) NOT NULL DEFAULT 'active',  -- active, paused, ended
    daily_budget_cents BIGINT NOT NULL DEFAULT 0,
    bid_cents       BIGINT NOT NULL DEFAULT 0,
    start_at        TIMESTAMPTZ,
    end_at          TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Ads: creatives per campaign
CREATE TABLE IF NOT EXISTS ads (
    id              SERIAL PRIMARY KEY,
    campaign_id     INTEGER NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    title           VARCHAR(255) NOT NULL,
    body            TEXT,
    image_url       VARCHAR(512),
    landing_url     VARCHAR(512) NOT NULL,
    targeting_criteria JSONB,  -- optional: segments, placements
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Events: impressions and clicks (for Phase 2+ aggregation)
CREATE TABLE IF NOT EXISTS events (
    id              BIGSERIAL PRIMARY KEY,
    request_id      VARCHAR(64),
    user_id         VARCHAR(64) NOT NULL,
    ad_id           INTEGER NOT NULL REFERENCES ads(id),
    campaign_id     INTEGER NOT NULL REFERENCES campaigns(id),
    event_type      VARCHAR(32) NOT NULL,  -- impression, click
    price_paid_cents BIGINT DEFAULT 0,
    context_json    JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_events_campaign_created ON events(campaign_id, created_at);
CREATE INDEX IF NOT EXISTS idx_events_type_created ON events(event_type, created_at);
CREATE INDEX IF NOT EXISTS idx_ads_campaign ON ads(campaign_id);
CREATE INDEX IF NOT EXISTS idx_campaigns_status ON campaigns(status);
