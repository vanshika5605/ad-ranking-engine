## Ad Ranking Engine

AI-powered ad ranking and second-price auction engine with a small end‑to‑end stack:

- **Ad server (Go + SQLite):** Fetches candidate ads, calls the ranking service, runs a second‑price auction, returns one winning ad, and logs impressions/clicks to Kafka.
- **Ranking service (Python/FastAPI):** Scores ads with predicted click‑through rate (pCTR) using a simple model (logistic regression) or a deterministic fallback.
- **Event consumer (Go):** Consumes impression/click events from Kafka and writes them to SQLite, maintaining daily per‑campaign stats.
- **Simulator (Python):** Generates synthetic traffic (ad requests + random clicks) to create training data and exercise the system.
- **Dashboard (React/Vite):** Shows campaigns, their ads, daily stats, and rule‑based optimization suggestions.

All persistent data (campaigns, ads, events, stats, model file) lives on a local SQLite DB and a shared model volume; there is no Postgres or Redis in the current implementation.

---

### How it works (high level)

- **When a request arrives** (`GET /v1/ads?user_id=...&placement=feed`):
  - The ad server loads eligible ads from SQLite (active campaigns, within start/end dates).
  - It calls the ranking service (`POST /rank`) to get a pCTR score for each ad (or uses a deterministic fallback if no model is loaded).
  - It computes **effective bid = bid_cents × score** and runs a **second‑price auction** on effective bids:
    - Highest effective bid wins.
    - Winner pays the second‑highest effective bid (or a small reserve price).
  - It returns the winning ad (creative + landing URL) and logs an **impression** event to Kafka.

- **When a click is logged** (`POST /v1/click`):
  - The ad server writes a **click** event to Kafka.

- **In the background**:
  - The event consumer reads impression/click events from Kafka, writes them to an `events` table in SQLite, and updates `campaign_stats_daily` (impressions, clicks, spend per campaign per day).

- **Training and ranking**:
  - A training job (`ranking-service/train.py`) reads events from SQLite, builds a dataset (impression vs click), trains a logistic regression CTR model, and saves it to `MODEL_PATH`.
  - The ranking service loads this model on startup (or on restart) and uses it to score ads; if there is no model, it falls back to a deterministic hash‑based pCTR so the system always works.

- **Dashboard and suggestions**:
  - The ad server exposes:
    - `GET /api/campaigns` — list of campaigns with aggregate stats and their ads.
    - `GET /api/campaigns/:id/stats` — daily stats for a single campaign.
    - `GET /api/suggestions` — simple rule‑based suggestions (e.g. “no traffic”, “low CTR & high spend”, “high CTR”).
  - The React dashboard calls these APIs and renders campaign tables, per‑campaign stats, and suggestions.

---

### Running the stack with Docker

From the repo root:

```bash
docker compose up -d --build
```

Basic smoke tests:

```bash
# Request an ad (logs an impression event)
curl "http://localhost:8080/v1/ads?user_id=user1&placement=feed"

# Log a click (use ad_id and campaign_id from the ad response)
curl -X POST http://localhost:8080/v1/click \
  -H "Content-Type: application/json" \
  -d '{"ad_id":1,"campaign_id":1,"request_id":"optional"}'

# Health checks
curl http://localhost:8080/health      # ad-server
curl http://localhost:8081/health      # ranking-service (includes model_loaded flag)
```

To run only the ad server (no Kafka/event‑consumer/ranking/dashboard):

```bash
docker compose up -d ad-server
# Or locally:
cd ad-server && go build && ./ad-server
# Uses DB_PATH=./data/adengine.db, and if KAFKA_BROKERS is unset, events are not sent to Kafka.
```

For detailed operational instructions (simulator, training job, logs, load tests), see `docs/RUNBOOK.md`.

---

### Project layout

- `ad-server/` — Go HTTP API, SQLite schema & seed, candidate fetch, ranking client, second‑price auction, Kafka producer, dashboard APIs.
- `ranking-service/` — Python FastAPI app (`main.py`) exposing `POST /rank`; `train.py` + `Dockerfile.train` for offline CTR training.
- `event-consumer/` — Go service: Kafka consumer → SQLite `events` + `campaign_stats_daily`.
- `simulator/` — Python script that simulates ad requests and random clicks to generate training data.
- `dashboard/` — React (Vite) UI for campaigns, ads, daily stats, and suggestions.
- `docs/RUNBOOK.md` — Runbook for start/stop, simulator, training, logs, load testing.
- `scripts/load_test.py` — Simple load test for `GET /v1/ads` (RPS and latency).
