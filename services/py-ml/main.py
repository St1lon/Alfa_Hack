"""py-ml-gatekeeper — СКЕЛЕТ.

Заведён Go #1, чтобы контур поднимался и остальные сервисы писались параллельно.
Содержимое — зона Python-разработчика (TEAM_PLAN §8): эмбеддинги, NER,
фильтр шума, okved_hints, склейка новостей в сюжеты.

Контракт эндпоинтов фиксируем сейчас, тела возвращают 501, пока не реализованы.
"""

import os

from fastapi import FastAPI
from fastapi.responses import JSONResponse
from pydantic import BaseModel

MODEL_VERSION = os.getenv("PYML_MODEL_VERSION", "pyml-v0")

app = FastAPI(title="py-ml-gatekeeper", version=MODEL_VERSION)


class EnrichRequest(BaseModel):
    """Запрос на обогащение одной новости."""

    news_id: str
    title: str
    body: str


class StoriesRequest(BaseModel):
    """Запрос на склейку обогащённых новостей в сюжеты."""

    news_ids: list[str]


@app.get("/healthz")
def healthz() -> dict[str, str]:
    """Liveness: процесс жив."""
    return {"status": "ok", "model_version": MODEL_VERSION}


@app.get("/readyz")
def readyz() -> dict[str, str]:
    """Readiness: модели загружены. Пока скелет — отвечаем ok."""
    return {"status": "ok"}


@app.post("/v1/enrich")
def enrich(_: EnrichRequest) -> JSONResponse:
    """Эмбеддинг, сущности, okved_hints, noise_score → news_enrichment."""
    return JSONResponse(
        status_code=501,
        content={
            "code": "NOT_IMPLEMENTED",
            "description": "эндпоинт зарезервирован контрактом, реализация в работе",
            "owner": "Python",
        },
    )


@app.post("/v1/stories")
def stories(_: StoriesRequest) -> JSONResponse:
    """Склейка перепечаток одной новости в один сюжет."""
    return JSONResponse(
        status_code=501,
        content={
            "code": "NOT_IMPLEMENTED",
            "description": "эндпоинт зарезервирован контрактом, реализация в работе",
            "owner": "Python",
        },
    )
