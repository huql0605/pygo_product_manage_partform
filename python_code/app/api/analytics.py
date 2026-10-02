from fastapi import APIRouter,Query
from app.modules.analytics import service

router = APIRouter()

@router.get("/sales")
def sales(days : int =Query(7 ,ge=1 ,le=365)):
    return service.get_sales_summary(days=days)

@router.get("/top-products")
def top_products(limit:int =Query(5,ge=1,le=100)):
    return service.get_top_products(limit=limit)
