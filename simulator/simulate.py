#!/usr/bin/env python3
"""
Simulate feed traffic: request ads and occasionally click.
Generates impressions + clicks in SQLite (via Kafka + event-consumer) for training.

Usage:
  AD_SERVER_URL=http://localhost:8080 python simulate.py [--requests 500] [--click-rate 0.03]
  Or: docker compose run simulator
"""
import argparse
import json
import random
import sys
import time
import urllib.error
import urllib.request

DEFAULT_URL = "http://localhost:8080"
DEFAULT_REQUESTS = 500
DEFAULT_CLICK_RATE = 0.03  # 3% of impressions become clicks
USER_IDS = [f"user_{i}" for i in range(1, 51)]  # 50 distinct users


def request_ad(base_url: str, user_id: str) -> dict | None:
    url = f"{base_url.rstrip('/')}/v1/ads?user_id={user_id}&placement=feed"
    req = urllib.request.Request(url)
    try:
        with urllib.request.urlopen(req, timeout=5) as r:
            return json.loads(r.read().decode())
    except (urllib.error.URLError, json.JSONDecodeError) as e:
        print(f"Request failed: {e}", file=sys.stderr)
        return None


def send_click(base_url: str, ad_id: int, campaign_id: int, user_id: str) -> bool:
    url = f"{base_url.rstrip('/')}/v1/click"
    data = json.dumps({
        "ad_id": ad_id,
        "campaign_id": campaign_id,
        "user_id": user_id,
    }).encode()
    req = urllib.request.Request(url, data=data, method="POST")
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=3) as r:
            return r.status == 200
    except urllib.error.URLError:
        return False


def main() -> None:
    p = argparse.ArgumentParser(description="Simulate ad requests and clicks")
    p.add_argument("--requests", "-n", type=int, default=DEFAULT_REQUESTS, help="Number of ad requests")
    p.add_argument("--click-rate", "-c", type=float, default=DEFAULT_CLICK_RATE, help="Probability of click per impression (0-1)")
    p.add_argument("--url", "-u", default=None, help=f"Ad server base URL (default: env AD_SERVER_URL or {DEFAULT_URL})")
    p.add_argument("--delay", "-d", type=float, default=0, help="Seconds between requests (default 0)")
    args = p.parse_args()
    base_url = args.url or __import__("os").environ.get("AD_SERVER_URL", DEFAULT_URL)

    impressions = 0
    clicks = 0
    errors = 0

    for i in range(args.requests):
        user_id = random.choice(USER_IDS)
        resp = request_ad(base_url, user_id)
        if resp is None:
            errors += 1
            continue
        ad = resp.get("ad")
        if not ad:
            continue
        impressions += 1
        ad_id = ad.get("ad_id")
        campaign_id = ad.get("campaign_id")
        if ad_id is None or campaign_id is None:
            continue
        if random.random() < args.click_rate:
            if send_click(base_url, ad_id, campaign_id, user_id):
                clicks += 1
        if args.delay > 0:
            time.sleep(args.delay)
        if (i + 1) % 100 == 0:
            print(f"  {i + 1}/{args.requests} requests, {impressions} impressions, {clicks} clicks", file=sys.stderr)

    print(f"Done: {impressions} impressions, {clicks} clicks, {errors} errors", file=sys.stderr)
    if impressions < 50:
        print("Tip: run with more requests (e.g. -n 500) so training has enough data.", file=sys.stderr)


if __name__ == "__main__":
    main()
