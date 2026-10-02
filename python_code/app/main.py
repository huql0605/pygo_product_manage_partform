from fastapi import FastAPI
from app.api import analytics

app = FastAPI(title="Analytics Service")

app.include_router(analytics.router , prefix="/analytics")

@app.get("/health")
def health():
    return {"status":"ok"}

