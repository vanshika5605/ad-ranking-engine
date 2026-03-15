-- Seed campaigns and ads for development
INSERT INTO campaigns (advertiser_id, name, status, daily_budget_cents, bid_cents, start_at, end_at)
VALUES
    ('adv_001', 'Summer Sale 2025', 'active', 10000, 50, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days'),
    ('adv_002', 'Brand Awareness Q1', 'active', 25000, 75, NOW() - INTERVAL '7 days', NOW() + INTERVAL '60 days'),
    ('adv_003', 'Product Launch', 'active', 5000, 30, NOW(), NOW() + INTERVAL '14 days');

INSERT INTO ads (campaign_id, title, body, image_url, landing_url)
VALUES
    (1, 'Summer Deals Inside', 'Up to 40% off on selected items. Limited time only.', 'https://example.com/img/summer.jpg', 'https://example.com/summer-sale'),
    (1, 'New Collection', 'Discover our latest arrivals.', 'https://example.com/img/new.jpg', 'https://example.com/new'),
    (2, 'Brand Story', 'Learn how we started and our mission.', 'https://example.com/img/brand.jpg', 'https://example.com/about'),
    (3, 'Launch Day', 'Be the first to try our new product.', 'https://example.com/img/launch.jpg', 'https://example.com/launch');
