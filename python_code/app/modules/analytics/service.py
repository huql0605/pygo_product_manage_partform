import pandas as pd
from app.db import engine

def get_sales_summary(days:int=7) ->dict:
    sql = """
SELECT 
    COUNT(*) AS product_count,
    COALESCE(SUM(sales), 0) AS total_sales,
    COALESCE(AVG(price), 0) AS avg_price,
    COALESCE(MAX(sales), 0) AS max_sales,
    COALESCE(MAX(price), 0) AS max_price
FROM products
"""
    df = pd.read_sql(sql,engine)

    row = df.iloc[0]

    return{
        "days":days,
        "product_count":int(row["product_count"]),
        "total_sales":int(row["total_sales"]),
        "avg_price": round(float(row["avg_price"]), 2),
        "max_sales": float(row["max_sales"]),
        "max_price": float(row["max_price"]),
    }

def get_top_products(limit:int = 5)->list[dict]:
    sql = """
    SELECT id, productor, name, price, sales
    FROM products
    ORDER BY price DESC
    LIMIT %(limit)s
    """
    df = pd.read_sql(sql,engine,params={"limit":limit})
    return df.to_dict(orient="records")