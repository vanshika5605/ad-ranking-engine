"""
Phase 4 training pipeline: read events from SQLite, train CTR model, export for ranking-service.
Run: DB_PATH=/path/to/adengine.db MODEL_PATH=/models/model.joblib python train.py
"""
import hashlib
import json
import os
import sqlite3
import sys
from pathlib import Path

import joblib
import numpy as np
from sklearn.linear_model import LogisticRegression

DB_PATH = os.getenv("DB_PATH", "./data/adengine.db")
MODEL_PATH = os.getenv("MODEL_PATH", "./models/model.joblib")
MIN_SAMPLES = 50  # skip training if too few events


def user_hash(user_id: str) -> int:
    """Same as ranking-service for consistent features."""
    h = hashlib.sha256(user_id.encode()).hexdigest()
    return int(h[:8], 16) % 1000


def build_features(ad_id: int, campaign_id: int, user_id: str) -> np.ndarray:
    return np.array([[float(ad_id), float(campaign_id), float(user_hash(user_id))]])


def main() -> None:
    if not Path(DB_PATH).exists():
        print(f"DB not found: {DB_PATH}", file=sys.stderr)
        sys.exit(1)

    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    cur = conn.execute(
        "SELECT ad_id, campaign_id, user_id, event_type FROM events"
    )
    rows = cur.fetchall()
    conn.close()

    if len(rows) < MIN_SAMPLES:
        print(f"Not enough events ({len(rows)} < {MIN_SAMPLES}), skip training", file=sys.stderr)
        sys.exit(0)

    X = []
    y = []
    for r in rows:
        ad_id = r["ad_id"]
        campaign_id = r["campaign_id"]
        user_id = r["user_id"] or "anonymous"
        X.append([float(ad_id), float(campaign_id), float(user_hash(user_id))])
        y.append(1 if r["event_type"] == "click" else 0)

    X = np.array(X)
    y = np.array(y)
    n_pos = int(y.sum())
    print(f"Training on {len(y)} events ({n_pos} clicks)", file=sys.stderr)

    model = LogisticRegression(
        class_weight="balanced",
        max_iter=500,
        random_state=42,
    )
    model.fit(X, y)

    Path(MODEL_PATH).parent.mkdir(parents=True, exist_ok=True)
    joblib.dump(model, MODEL_PATH)

    meta = {
        "version": 1,
        "n_samples": len(y),
        "n_clicks": n_pos,
        "model_path": MODEL_PATH,
    }
    meta_path = str(Path(MODEL_PATH).with_suffix(".meta.json"))
    with open(meta_path, "w") as f:
        json.dump(meta, f, indent=2)

    print(f"Saved model to {MODEL_PATH}", file=sys.stderr)


if __name__ == "__main__":
    main()
