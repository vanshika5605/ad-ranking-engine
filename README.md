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

## Phase 4 (Training Pipeline)

- **Training job:** Reads `events` from SQLite, trains a logistic regression (CTR: click vs impression), saves model to a shared volume.
- **Ranking service:** Loads model from `MODEL_PATH` if present and uses it for scoring; otherwise uses the deterministic fallback.
- **Generate traffic for training:** Run the simulator to create impressions and clicks (event-consumer writes them to SQLite).  
  **Option A – Docker:** `docker compose --profile simulate run simulator` (500 requests, ~3% click rate).  
  **Option B – Local:** `AD_SERVER_URL=http://localhost:8080 python simulator/simulate.py -n 500 -c 0.03`  
  Override: `docker compose run simulator -- --requests 1000 --click-rate 0.05` or `simulate.py -n 1000 -c 0.05`.
- **Run training (after you have ≥50 events):**  
  `docker compose --profile train run train`  
  Then restart ranking-service to pick up the new model:  
  `docker compose restart ranking-service`

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
- `ranking-service/` — Python FastAPI: `POST /rank` returns pCTR scores; loads trained model from volume or uses fallback. `train.py` + `Dockerfile.train` for offline training.
- `event-consumer/` — Go: Kafka → SQLite events + campaign_stats_daily
- `simulator/` — Python script: simulates ad requests + random clicks for training data
- `dashboard/` — React (Vite) app: campaign list, daily stats, suggestions
- `docs/PLAN.md` — Full build plan and phases

## Phase 5 (Dashboard)

- **Dashboard API** on ad-server: `GET /api/campaigns`, `GET /api/campaigns/:id/stats`, `GET /api/suggestions`.
- **React dashboard** (Vite): campaign list with metrics, daily stats per campaign, optimization suggestions.
- **Run:** `docker compose up -d dashboard` then open http://localhost:3000. Or locally: `cd dashboard && npm install && npm run dev` (proxy to ad-server on 8080).

Later phases: deploy & harden.
