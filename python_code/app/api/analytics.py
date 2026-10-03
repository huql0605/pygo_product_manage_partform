from fastapi import APIRouter,Query
from app.modules.analytics import service

router = APIRouter()

@router.get("/sales")
def sales(
    start_month :str | None = Query(None,description="2024-01"),
    end_month :str | None = Query(None,description="2024-12"),
    ):
    return service.get_sales_summary(start_month=start_month,end_month=end_month)

@router.get("/top-products")
def top_products(limit:int =Query(5,ge=1,le=500)):
    return service.get_top_products(limit=limit)

@router.get("/monthly-trend")
def monthly_trend(productor : str | None = Query(None)):
    return service.get_monthly_trend(productor=productor)

@router.get("/brands")
def brands(limit: int =Query(10,ge=1,le=50)):
    return service.get_brand_ranking(limit=limit)
