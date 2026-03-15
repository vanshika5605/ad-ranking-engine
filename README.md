# Ad Ranking Engine

AI-powered ad ranking and second-price auction engine. See [docs/PLAN.md](docs/PLAN.md) for the full implementation plan and phases.

## Phase 1 (Foundation)

- **Data:** SQLite (single file: campaigns, ads, events). No DB server required.
- **Ad server:** Go service that fetches candidate ads and returns one (simple auction: highest bid wins).

### Run locally

**Option A – Docker**

```bash
docker compose up -d ad-server
curl "http://localhost:8080/v1/ads?user_id=user1&placement=feed"
curl http://localhost:8080/health
```

**Option B – Local (no Docker)**

```bash
cd ad-server
go build -o ad-server .
./ad-server
# DB file: ./data/adengine.db (or set DB_PATH)
```

### Project layout

- `ad-server/` — Go HTTP API + SQLite schema/seed (`schema.sql`, `seed.sql`)
- `docs/PLAN.md` — Full build plan and phases

Later phases add: second-price auction, Kafka events, ranking service (ML), dashboard, deploy.
