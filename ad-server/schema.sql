-- SQLite schema for ad-ranking-engine
CREATE TABLE IF NOT EXISTS campaigns (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    advertiser_id        TEXT NOT NULL,
    name                TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'active',
    daily_budget_cents   INTEGER NOT NULL DEFAULT 0,
    bid_cents           INTEGER NOT NULL DEFAULT 0,
    start_at            TEXT,
    end_at              TEXT,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS ads (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    campaign_id          INTEGER NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    title               TEXT NOT NULL,
    body                TEXT,
    image_url           TEXT,
    landing_url         TEXT NOT NULL,
    targeting_criteria   TEXT,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS events (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id          TEXT,
    user_id             TEXT NOT NULL,
    ad_id               INTEGER NOT NULL REFERENCES ads(id),
    campaign_id          INTEGER NOT NULL REFERENCES campaigns(id),
    event_type          TEXT NOT NULL,
    price_paid_cents    INTEGER DEFAULT 0,
    context_json        TEXT,
    created_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Aggregated stats per campaign per day (updated by event-consumer)
CREATE TABLE IF NOT EXISTS campaign_stats_daily (
    campaign_id      INTEGER NOT NULL REFERENCES campaigns(id),
    date             TEXT NOT NULL,
    impressions      INTEGER NOT NULL DEFAULT 0,
    clicks           INTEGER NOT NULL DEFAULT 0,
    spend_cents      INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (campaign_id, date)
);

CREATE INDEX IF NOT EXISTS idx_events_campaign_created ON events(campaign_id, created_at);
CREATE INDEX IF NOT EXISTS idx_events_type_created ON events(event_type, created_at);
CREATE INDEX IF NOT EXISTS idx_ads_campaign ON ads(campaign_id);
CREATE INDEX IF NOT EXISTS idx_campaigns_status ON campaigns(status);
