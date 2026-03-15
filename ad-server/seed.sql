-- Seed campaigns and ads (SQLite)
INSERT OR IGNORE INTO campaigns (id, advertiser_id, name, status, daily_budget_cents, bid_cents, start_at, end_at)
VALUES
    (1, 'adv_001', 'Summer Sale 2025', 'active', 10000, 50, datetime('now', '-1 day'), datetime('now', '+30 days')),
    (2, 'adv_002', 'Brand Awareness Q1', 'active', 25000, 75, datetime('now', '-7 days'), datetime('now', '+60 days')),
    (3, 'adv_003', 'Product Launch', 'active', 5000, 30, datetime('now'), datetime('now', '+14 days'));

INSERT OR IGNORE INTO ads (campaign_id, title, body, image_url, landing_url)
VALUES
    (1, 'Summer Deals Inside', 'Up to 40% off on selected items. Limited time only.', 'https://example.com/img/summer.jpg', 'https://example.com/summer-sale'),
    (1, 'New Collection', 'Discover our latest arrivals.', 'https://example.com/img/new.jpg', 'https://example.com/new'),
    (2, 'Brand Story', 'Learn how we started and our mission.', 'https://example.com/img/brand.jpg', 'https://example.com/about'),
    (3, 'Launch Day', 'Be the first to try our new product.', 'https://example.com/img/launch.jpg', 'https://example.com/launch');
