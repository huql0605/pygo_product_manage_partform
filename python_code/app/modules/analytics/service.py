import pandas as pd
from app.db import engine

def get_sales_summary(start_month : str | None = None , end_month :str | None = None) ->dict:
    where , params =[] , {}
    if start_month:
        where.append("month >= %(start_month)s")
        params["start_month"]= start_month
    if end_month:
        where.append("month <= %(end_month)s")
        params["end_month"]= end_month
    where_sql =f"WHERE {' AND '.join(where)}" if where else ""

    sql = f"""
        SELECT
            COUNT(DISTINCT productor)      AS brand_count,
            COUNT(DISTINCT name)           AS product_count,
            COALESCE(SUM(sales), 0)        AS total_sales,
            COALESCE(SUM(price * sales),0) AS total_amount,
            COALESCE(AVG(price), 0)        AS avg_price
        FROM sales
        {where_sql}
    """
    df = pd.read_sql(sql,engine,params=params)
    row =df.iloc[0]
    return {
        "start_month":start_month,
        "end_month":end_month,
        "brand_count":int(row["brand_count"]),
        "product_count":int(row["product_count"]),
        "total_sales":int(row["total_sales"]),
        "total_amount":round(float(row["total_amount"]),2),
        "avg_price":round(float(row["avg_price"]),2),
    }


def get_brand_ranking(limit:int = 5)->list[dict]:
    sql = """
    SELECT
        productor                 AS brand,
        SUM(sales)                AS total_sales,
        SUM(price * sales)        AS total_amount,
        COUNT(DISTINCT name)      AS product_count
    FROM sales
    GROUP BY productor
    ORDER BY total_sales DESC
    LIMIT %(limit)s
    """
    df = pd.read_sql(sql,engine,params={"limit":limit})
    return df.to_dict(orient="records")

def get_top_products(limit : int =5) ->list[dict]:
    sql = """
        SELECT
            productor                 AS brand,
            name,
            SUM(sales)                AS total_sales,
            SUM(price * sales)        AS total_amount,
            AVG(price)                AS avg_price
        FROM sales
        GROUP BY productor, name
        ORDER BY total_sales DESC
        LIMIT %(limit)s
    """
    df = pd.read_sql(sql,engine,params={"limit":limit})
    return df.to_dict(orient="records")

def get_monthly_trend(productor : str |None = None) -> list[dict]:
    if productor:
        sql="""
            SELECT month ,SUM(sales) AS total_sales , SUM(price * sales) AS total_amount
            FROM sales
            WHERE productor = %(productor)s
            GROUP BY month 
            ORDER BY month
        """
        params={"productor":productor}

    else:
        sql="""
            SELECT month,SUM(sales) AS total_sales , SUM(price * sales) AS total_amount
            FROM sales
            GROUP BY month
            ORDER BY month
        """
        params ={}

    df = pd.read_sql(sql , engine ,params=params)

    return df.to_dict(orient="records")

