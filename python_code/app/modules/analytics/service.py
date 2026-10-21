import pandas as pd
from app.db import engine

class CategoryNotFoundError(LookupError):
    def __init__(self, category:str) -> None:
        self.category=category
        super().__init__(f"未知品类: {category}") 

def _resolve_category(category:str | None) ->int | None:
    if not category:
        return None
    df = pd.read_sql(
        "SELECT id FROM products WHERE name = %(category)s",
        engine,
        params={"category":category},
    )
    if df.empty:
        raise CategoryNotFoundError(category=category)
    return int(df.iloc[0]["id"])

def _filters(category=None,productor=None,start_month=None,end_month=None) :
    where , params = [] , {}

    product_id = _resolve_category(category=category)
    if product_id is not None:
        where.append("product_id = %(product_id)s")
        params["product_id"]= product_id
    if productor :
        where.append("productor = %(productor)s")
        params["productor"]= productor
    if start_month :
        where.append("month >= %(start_month)s")
        params["start_month"]= start_month
    if end_month :
        where.append("month <= %(end_month)s")
        params["end_month"]= end_month

    where_sql =f"WHERE {' AND '.join(where)}" if where else ""
    return where_sql,params

def get_sales_summary(start_month : str | None = None , end_month :str | None = None,category = None) ->dict:
    where_sql,params = _filters(category=category,start_month=start_month,end_month=end_month)

    sql = f"""
        SELECT
            COUNT(DISTINCT productor)                                 AS brand_count,
            COUNT(DISTINCT name)                                      AS product_count,
            COALESCE(SUM(total_sales), 0)                             AS total_sales,
            COALESCE(SUM(total_amount),0)                             AS total_amount,
            COALESCE(SUM(price_sum) / NULLIF(SUM(record_count),0), 0) AS avg_price
        FROM dws_sales
        {where_sql}
    """
    row = pd.read_sql(sql,engine,params=params).iloc[0]
    return {
        "category":category,
        "start_month":start_month,
        "end_month":end_month,
        "brand_count":int(row["brand_count"]),
        "product_count":int(row["product_count"]),
        "total_sales":int(row["total_sales"]),
        "total_amount":round(float(row["total_amount"]),2),
        "avg_price":round(float(row["avg_price"]),2),
    }


def get_brand_ranking(limit:int = 5,category:str | None = None)->list[dict]:
    where_sql , params =_filters(category=category)
    params["limit"]=limit
    sql = f"""
        SELECT productor              AS brand,
               SUM(total_sales)       AS total_sales,
               SUM(total_amount)      AS total_amount,
               COUNT(DISTINCT name)   AS product_count
        FROM dws_sales
        {where_sql}
        GROUP BY productor
        ORDER BY total_sales DESC
        LIMIT %(limit)s
    """
    return pd.read_sql(sql, engine, params=params).to_dict(orient="records")

def get_top_products(limit: int = 5, category: str | None = None):
    where_sql, params = _filters(category=category)
    params["limit"] = limit
    sql = f"""
        SELECT productor AS brand,
               name,
               SUM(total_sales)                             AS total_sales,
               SUM(total_amount)                            AS total_amount,
               SUM(price_sum) / NULLIF(SUM(record_count),0) AS avg_price
        FROM dws_sales
        {where_sql}
        GROUP BY productor, name
        ORDER BY total_sales DESC
        LIMIT %(limit)s
    """
    return pd.read_sql(sql, engine, params=params).to_dict(orient="records")


def get_monthly_trend(productor: str | None = None, category: str | None = None):
    where_sql, params = _filters(category=category, productor=productor)
    sql = f"""
        SELECT month,
               SUM(total_sales)  AS total_sales,
               SUM(total_amount) AS total_amount
        FROM dws_sales
        {where_sql}
        GROUP BY month
        ORDER BY month
    """
    return pd.read_sql(sql, engine, params=params).to_dict(orient="records")

