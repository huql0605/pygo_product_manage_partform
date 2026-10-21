from fastapi import FastAPI
from fastapi.responses import JSONResponse
from app.modules.analytics.service import CategoryNotFoundError
from starlette.middleware.base import BaseHTTPMiddleware
from app.api import importer
from app.api import analytics
from app.core.logging import request_id_var

app = FastAPI(title="Analytics Service")


class RequestIDMiddleware(BaseHTTPMiddleware):
    """把 Go 透传过来的 X-Request-ID 存进 ContextVar,供日志使用。"""

    async def dispatch(self, request, call_next):
        rid = request.headers.get("X-Request-ID", "-")
        request_id_var.set(rid)
        response = await call_next(request)
        response.headers["X-Request-ID"] = rid
        return response
    
@app.exception_handler(CategoryNotFoundError)
async def _category_not_found(request,exc):
    return JSONResponse(
        status_code=404,
        content={"code":404,"msg":str(exc),"data":None}
    )

app.add_middleware(RequestIDMiddleware)

app.include_router(analytics.router , prefix="/analytics")

app.include_router(importer.router,prefix="/import")

@app.get("/health")
def health():
    return {"status":"ok"}

