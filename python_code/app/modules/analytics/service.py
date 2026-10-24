import re

import pandas as pd

from app.config import settings
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

def get_sales_summary(start_month : str | None = None , end_month :str | None = None,category = None,productor : str | None = None) ->dict:
    where_sql,params = _filters(category=category,productor=productor,start_month=start_month,end_month=end_month)

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


def get_brand_ranking(limit:int = 5,category:str | None = None,start_month:str | None = None,end_month:str | None = None,productor:str | None = None)->list[dict]:
    where_sql , params =_filters(category=category,productor=productor,start_month=start_month,end_month=end_month)
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

def get_top_products(limit: int = 5, category: str | None = None, start_month: str | None = None, end_month: str | None = None, productor: str | None = None):
    where_sql, params = _filters(category=category, productor=productor, start_month=start_month, end_month=end_month)
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


def get_monthly_trend(productor: str | None = None, category: str | None = None, start_month: str | None = None, end_month: str | None = None):
    where_sql, params = _filters(category=category, productor=productor, start_month=start_month, end_month=end_month)
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


# ---------- 属性聚合（探索口径） ----------

_KEY_RE = re.compile(r"^[A-Za-z0-9_]{1,40}$")


class AttributeNotAllowedError(LookupError):
    """请求的属性不在白名单内。属性聚合不接受任意 key，避免开放式扫描。"""

    def __init__(self, key: str) -> None:
        self.key = key
        super().__init__(f"未登记的属性: {key!r}")


def _allowed_attr_keys() -> tuple[bool, set[str]]:
    """返回 (是否放行任意 key, 允许的 key 集合)。

    ALLOWED_ATTRS 设为 * 表示放行任意符合命名规则的 key。
    """
    raw = (settings.allowed_attrs or "").strip()
    if raw == "*":
        return True, set()
    return False, {k.strip() for k in raw.split(",") if k.strip()}


def _check_attr_key(key: str | None) -> str:
    key = (key or "").strip()
    if not _KEY_RE.fullmatch(key):
        raise AttributeNotAllowedError(key)
    wildcard, allowed = _allowed_attr_keys()
    if not wildcard and key not in allowed:
        raise AttributeNotAllowedError(key)
    return key


def get_attribute_breakdown(
    key: str,
    limit: int = 20,
    category: str | None = None,
    start_month: str | None = None,
    end_month: str | None = None,
) -> dict:
    """按某个属性分组统计销量与金额。

    与 /analytics/* 的区别：
    - 数据源是 sales 明细，不是 dws_sales
    - 属于探索口径，属性没有被纳入汇总层粒度
    - key 必须命中白名单，分组维度只有属性本身
    """
    key = _check_attr_key(key)

    where_sql, params = _filters(
        category=category, start_month=start_month, end_month=end_month
    )
    conditions = ["attrs ? %(key)s"]
    if where_sql:
        conditions.append(where_sql.removeprefix("WHERE "))
    where_clause = "WHERE " + " AND ".join(conditions)

    params["key"] = key
    params["limit"] = limit

    sql = f"""
        SELECT COALESCE(attrs->>%(key)s, '(未标注)') AS value,
               COALESCE(SUM(sales), 0)               AS total_sales,
               COALESCE(SUM(price * sales), 0)       AS total_amount,
               COUNT(*)                              AS record_count
        FROM sales
        {where_clause}
        GROUP BY 1
        ORDER BY total_sales DESC
        LIMIT %(limit)s
    """
    df = pd.read_sql(sql, engine, params=params)

    items = df.to_dict(orient="records")
    for item in items:
        item["total_sales"] = int(item["total_sales"])
        item["total_amount"] = round(float(item["total_amount"]), 2)
        item["record_count"] = int(item["record_count"])

    return {
        "key": key,
        "category": category,
        "start_month": start_month,
        "end_month": end_month,
        "items": items,
    }

