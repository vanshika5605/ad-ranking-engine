# Ad Ranking Engine

AI-powered ad ranking and second-price auction engine. See [docs/PLAN.md](docs/PLAN.md) for the full implementation plan and phases.

## Phase 1 (Foundation)

- **Data:** SQLite (campaigns, ads, events). No DB server required.
- **Ad server:** Fetches candidates, runs auction, returns one ad.

## Phase 2 (Auction & Events)

- **Second-price auction:** Winner pays second-highest bid (or reserve).
- **Kafka:** Ad-server produces impression and click events to topics `impressions` and `clicks`.
- **Event-consumer:** Consumes from Kafka, writes to SQLite (`events` table) and updates `campaign_stats_daily`.

## Phase 3 (Ranking & ML)

- **Ranking service (Python/FastAPI):** `POST /rank` returns pCTR scores per ad; ad-server calls it before the auction.
- **Auction:** Uses effective bid = `bid_cents * score`; second-price on effective bid.
- If the ranking service is down or not configured, ad-server falls back to score = 1.0 (bid-only).

### Run with Docker

```bash
docker compose up -d
# Request an ad (logs impression to Kafka; event-consumer writes to SQLite)
curl "http://localhost:8080/v1/ads?user_id=user1&placement=feed"
# Log a click (send request_id, ad_id, campaign_id from the ad response)
curl -X POST http://localhost:8080/v1/click -H "Content-Type: application/json" -d '{"ad_id":1,"campaign_id":1,"request_id":"optional"}'
curl http://localhost:8080/health
```

### Run ad-server only (no Kafka)

```bash
docker compose up -d ad-server
# Or locally: cd ad-server && go build && ./ad-server
# DB_PATH=./data/adengine.db, no KAFKA_BROKERS → impressions/clicks not sent to Kafka
```

### Project layout

- `ad-server/` — Go HTTP API, SQLite, second-price auction (effective bid = bid × pCTR), Kafka producer, ranking client
- `ranking-service/` — Python FastAPI: `POST /rank` returns pCTR scores (simple model; Phase 4 adds training)
- `event-consumer/` — Go: Kafka → SQLite events + campaign_stats_daily
- `docs/PLAN.md` — Full build plan and phases

Later phases: training pipeline, dashboard, deploy.
