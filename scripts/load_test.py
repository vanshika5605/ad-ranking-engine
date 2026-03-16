#!/usr/bin/env python3
"""
Load test for GET /v1/ads. Reports RPS and latency percentiles.
Usage: python scripts/load_test.py [--url URL] [--duration D] [--concurrency C]
"""
import argparse
import statistics
import threading
import time
import urllib.request

def hit(url: str, results: list, errors: list) -> None:
    start = time.perf_counter()
    try:
        req = urllib.request.Request(url)
        with urllib.request.urlopen(req, timeout=10) as r:
            r.read()
        results.append((time.perf_counter() - start) * 1000)
    except Exception as e:
        errors.append(str(e))


def main() -> None:
    p = argparse.ArgumentParser(description="Load test GET /v1/ads")
    p.add_argument("--url", "-u", default="http://localhost:8080/v1/ads?user_id=loadtest", help="Full URL to hit")
    p.add_argument("--duration", "-d", type=float, default=10, help="Seconds to run")
    p.add_argument("--concurrency", "-c", type=int, default=10, help="Concurrent threads")
    args = p.parse_args()

    results = []
    errors = []
    stop = threading.Event()
    threads = []

    def worker():
        while not stop.is_set():
            hit(args.url, results, errors)

    for _ in range(args.concurrency):
        t = threading.Thread(target=worker)
        t.start()
        threads.append(t)

    time.sleep(args.duration)
    stop.set()
    for t in threads:
        t.join()

    total = len(results) + len(errors)
    rps = total / args.duration if args.duration else 0
    print(f"Duration: {args.duration}s | Concurrency: {args.concurrency}")
    print(f"Total requests: {total} | Errors: {len(errors)} | RPS: {rps:.1f}")
    if results:
        results.sort()
        n = len(results)
        p50 = results[int(n * 0.5)] if n else 0
        p95 = results[int(n * 0.95)] if n > 1 else (results[0] if results else 0)
        p99 = results[int(n * 0.99)] if n > 1 else (results[0] if results else 0)
        print(f"Latency (ms) — p50: {p50:.1f} | p95: {p95:.1f} | p99: {p99:.1f} | mean: {statistics.mean(results):.1f}")
    if errors:
        sample = list(set(errors))[:3]
        print(f"Error sample: {sample}")


if __name__ == "__main__":
    main()
