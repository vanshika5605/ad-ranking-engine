"""
Ranking service: scores ad candidates (pCTR) for the ad-server auction.
POST /rank accepts candidates + context, returns (ad_id, score) for each.
"""
import hashlib
import os
from typing import Any

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

app = FastAPI(title="Ad Ranking Service", version="0.1.0")

# Optional: Redis for feature lookup (Phase 3 minimal: no Redis, compute from request)
REDIS_URL = os.getenv("REDIS_URL", "")


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


def _pctr_score(ad_id: int, campaign_id: int, user_id: str) -> float:
    """
    Simple deterministic pCTR: varies by ad and user so auction order differs from bid-only.
    In Phase 4 we can replace this with a trained model (e.g. logistic regression).
    """
    h = hashlib.sha256(f"{ad_id}:{campaign_id}:{user_id}".encode()).hexdigest()
    # Score in [0.008, 0.025] so effective_bid = bid * score is meaningful
    base = 0.008
    spread = (int(h[:8], 16) % 17000) / 1_000_000  # ~0-0.017
    return round(base + spread, 6)


@app.post("/rank", response_model=RankResponse)
def rank(request: RankRequest) -> RankResponse:
    if not request.candidates:
        return RankResponse(scores=[])
    context = request.context or {}
    user_id = context.get("user_id", "anonymous")
    scores = [
        ScoreOutput(ad_id=c.ad_id, score=_pctr_score(c.ad_id, c.campaign_id, user_id))
        for c in request.candidates
    ]
    return RankResponse(scores=scores)


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}
