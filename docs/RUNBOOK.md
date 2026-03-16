# Ad Ranking Engine — Runbook

Quick reference for running and operating the stack locally (Docker Compose).

---

## Start the stack

```bash
# From repo root
docker compose up -d --build
```

**Services and ports:**

| Service          | Port  | Purpose                          |
|------------------|-------|----------------------------------|
| ad-server        | 8080  | Ad API, dashboard API            |
| ranking-service  | 8081  | pCTR scoring                     |
| dashboard        | 3000  | React UI                         |
| Kafka            | 9092  | Event streaming                  |
| Redis            | 6379  | (Reserved for future use)        |

**Health checks:**

```bash
curl -s http://localhost:8080/health   # ad-server
curl -s http://localhost:8081/health   # ranking-service (includes model_loaded)
```

---

## Stop the stack

```bash
docker compose down
```

To remove volumes (resets DB and model):

```bash
docker compose down -v
```

---

## Generate traffic (simulator)

```bash
# 500 requests, ~3% click rate (default)
docker compose run simulator

# Custom: 1000 requests, 5% click rate
docker compose run simulator -- --requests 1000 --click-rate 0.05
```

Events are written to SQLite via Kafka and the event-consumer. Allow a few seconds for events to appear.

---

## Train the CTR model

1. Ensure you have at least ~50 events (run the simulator first if needed).
2. Run the training job:

   ```bash
   docker compose --profile train run train
   ```

3. Restart the ranking service so it loads the new model:

   ```bash
   docker compose restart ranking-service
   ```

4. Confirm model is loaded:

   ```bash
   curl -s http://localhost:8081/health
   # Expect "model_loaded": true
   ```

---

## View logs

```bash
# All services
docker compose logs -f

# Single service (last 100 lines, then follow)
docker compose logs -f ad-server
docker compose logs -f event-consumer
docker compose logs -f ranking-service
docker compose logs -f dashboard
```

---

## Restart a single service

```bash
docker compose restart ad-server
docker compose restart ranking-service
docker compose restart event-consumer
docker compose restart dashboard
```

After changing code, rebuild and restart:

```bash
docker compose up -d --build ad-server
```

---

## Common issues

**Dashboard shows "Failed to fetch" or no campaigns**

- Ensure ad-server is up: `curl http://localhost:8080/health`
- If using the Docker dashboard, open http://localhost:3000 on the same machine where Docker runs (API is hardcoded to localhost:8080 in the build).

**500 errors when running the simulator**

- Ad-server and event-consumer both use SQLite; `PRAGMA busy_timeout` is set. If 500s persist, run the simulator with a small delay: `docker compose run simulator -- --delay 0.02`

**No suggestions on dashboard**

- Campaigns with 0 impressions get a "no traffic" suggestion. Other suggestions require enough traffic and variance (e.g. low CTR + high spend). Run the simulator to generate more data.

**Training says "Not enough events"**

- Run the simulator first to create impressions and clicks, then run the train job again.

---

## Load test

From repo root, with the stack running:

```bash
# Default: 10s, 10 concurrent threads, GET /v1/ads
python scripts/load_test.py

# Custom duration and concurrency
python scripts/load_test.py --duration 15 --concurrency 20

# Custom URL (e.g. different host/port)
python scripts/load_test.py --url "http://localhost:8080/v1/ads?user_id=user1&placement=feed"
```

Output: total requests, errors, RPS, and latency percentiles (p50, p95, p99, mean in ms).

**Example results (10s duration, stack running in Docker):**

| Concurrency | Total requests | RPS  | p50 (ms) | p95 (ms) | p99 (ms) | Mean (ms) |
|-------------|----------------|------|----------|----------|----------|-----------|
| 5           | 473            | 47.3 | 28.8     | 355.0    | 542.4    | 107.9     |
| 10          | 3,421          | 342.1| 6.5      | 195.8    | 252.8    | 29.6      |
| 20          | 1,039          | 103.9| 166.7    | 476.6    | 1688.4   | 195.8     |
| 50          | 1,579          | 157.9| 221.5    | 410.8    | 4476.4   | 321.9     |

Results vary by hardware, Docker resource limits, and whether Kafka/ranking-service are under load. Higher concurrency can increase tail latency (p95/p99) due to SQLite and ranking-service contention.
