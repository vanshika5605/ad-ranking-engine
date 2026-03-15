# Ad Ranking Engine

AI-powered ad ranking and second-price auction engine. See [docs/PLAN.md](docs/PLAN.md) for the full implementation plan and phases.

## Phase 1 (Foundation)

- **Data:** Postgres (campaigns, ads, events schema) + Redis + Kafka in Docker
- **Ad server:** Go service that fetches candidate ads and returns one (simple auction: highest bid wins)

### Run locally

1. Start infrastructure and ad-server:

   ```bash
   docker compose up -d postgres redis zookeeper kafka
   # wait for Postgres to be healthy, then:
   docker compose up -d ad-server
   ```

2. Request an ad:

   ```bash
   curl "http://localhost:8080/v1/ads?user_id=user1&placement=feed"
   ```

3. Health check:

   ```bash
   curl http://localhost:8080/health
   ```

### Project layout

- `ad-server/` — Go HTTP API (candidate retrieval + auction)
- `postgres/init/` — Schema and seed data
- `docs/PLAN.md` — Full build plan and phases

Later phases add: second-price auction, Kafka events, ranking service (ML), dashboard, deploy.
