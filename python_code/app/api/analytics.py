from fastapi import APIRouter,Query
from app.modules.analytics import service

router = APIRouter()

@router.get("/sales")
def sales(
    start_month :str | None = Query(None,description="2024-01"),
    end_month :str | None = Query(None,description="2024-12"),
    category : str | None = Query(None,description="品类"),
    productor : str | None = Query(None,description="品牌")
    ):
    return service.get_sales_summary(
        start_month=start_month,
        end_month=end_month,
        category=category,
        productor=productor,
    )

@router.get("/top-products")
def top_products(limit:int =Query(5,ge=1,le=500),
                 category : str| None = Query(None),
                 start_month : str| None = Query(None,description="2024-01"),
                 end_month : str| None = Query(None,description="2024-12"),
                 productor : str| None = Query(None)):
    return service.get_top_products(
        limit=limit,
        category=category,
        start_month=start_month,
        end_month=end_month,
        productor=productor,
    )

@router.get("/monthly-trend")
def monthly_trend(productor : str | None = Query(None),
                  category :str |None = Query(None),
                  start_month : str|None = Query(None,description="2024-01"),
                  end_month : str|None = Query(None,description="2024-12")):
    return service.get_monthly_trend(
        productor=productor,
        category=category,
        start_month=start_month,
        end_month=end_month,
    )

@router.get("/brands")
def brands(limit: int =Query(5,ge=1,le=50),
           category : str|None = Query(None),
           start_month : str|None = Query(None,description="2024-01"),
           end_month : str|None = Query(None,description="2024-12"),
           productor : str|None = Query(None)):
    return service.get_brand_ranking(
        limit=limit,
        category=category,
        start_month=start_month,
        end_month=end_month,
        productor=productor,
    )

@router.get("/attributes")
def attributes(key : str = Query(...,description="属性名，必须在白名单内"),
               limit: int =Query(20,ge=1,le=100),
               category : str|None = Query(None),
               start_month : str|None = Query(None,description="2024-01"),
               end_month : str|None = Query(None,description="2024-12")):
    return service.get_attribute_breakdown(
        key=key,
        limit=limit,
        category=category,
        start_month=start_month,
        end_month=end_month,
    )
