"""
Ranking service: scores ad candidates (pCTR) for the ad-server auction.
POST /rank accepts candidates + context, returns (ad_id, score) for each.
Loads trained model from MODEL_PATH if set (Phase 4); else uses deterministic fallback.
"""
import hashlib
import os
from pathlib import Path
from typing import Any

import joblib
import numpy as np
from fastapi import FastAPI
from pydantic import BaseModel

app = FastAPI(title="Ad Ranking Service", version="0.1.0")

MODEL_PATH = os.getenv("MODEL_PATH", "")
_model = None


def _user_hash(user_id: str) -> int:
    """Must match train.py for consistent features."""
    h = hashlib.sha256(user_id.encode()).hexdigest()
    return int(h[:8], 16) % 1000


def _load_model() -> Any:
    global _model
    if _model is not None:
        return _model
    if not MODEL_PATH or not Path(MODEL_PATH).exists():
        return None
    try:
        _model = joblib.load(MODEL_PATH)
        return _model
    except Exception:
        return None


def _pctr_fallback(ad_id: int, campaign_id: int, user_id: str) -> float:
    """Deterministic pCTR when no model is loaded."""
    h = hashlib.sha256(f"{ad_id}:{campaign_id}:{user_id}".encode()).hexdigest()
    base = 0.008
    spread = (int(h[:8], 16) % 17000) / 1_000_000
    return round(base + spread, 6)


def _score_one(ad_id: int, campaign_id: int, user_id: str) -> float:
    model = _load_model()
    if model is None:
        return _pctr_fallback(ad_id, campaign_id, user_id)
    X = np.array([[float(ad_id), float(campaign_id), float(_user_hash(user_id))]])
    try:
        proba = model.predict_proba(X)[0, 1]
        return max(0.001, min(0.5, float(proba)))
    except Exception:
        return _pctr_fallback(ad_id, campaign_id, user_id)


class CandidateInput(BaseModel):
    ad_id: int
    campaign_id: int
    bid_cents: int


class RankRequest(BaseModel):
    candidates: list[CandidateInput]
    context: dict[str, Any] | None = None


class ScoreOutput(BaseModel):
    ad_id: int
    score: float


class RankResponse(BaseModel):
    scores: list[ScoreOutput]


@app.post("/rank", response_model=RankResponse)
def rank(request: RankRequest) -> RankResponse:
    if not request.candidates:
        return RankResponse(scores=[])
    context = request.context or {}
    user_id = context.get("user_id", "anonymous")
    scores = [
        ScoreOutput(ad_id=c.ad_id, score=_score_one(c.ad_id, c.campaign_id, user_id))
        for c in request.candidates
    ]
    return RankResponse(scores=scores)


@app.get("/health")
def health() -> dict[str, Any]:
    loaded = _load_model() is not None
    return {"status": "ok", "model_loaded": loaded}
