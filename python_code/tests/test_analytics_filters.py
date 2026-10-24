"""分析层过滤条件的单元测试。

运行：
    cd python_code
    python -m unittest discover -s tests -v

这些用例专门钉住曾经踩过的两个 bug：
1. 条件写成 `if product_id is None`，导致带品类时反而不过滤
2. 条件写成 `if productor is not None`，导致空字符串也参与过滤
"""

import os
import unittest
from unittest.mock import patch

# app.config 在导入时就会读取环境变量，先给一份安全默认值，
# 保证测试不依赖本地 .env。这里不会真正建立数据库连接。
os.environ.setdefault("DATABASE_URL", "postgresql+psycopg2://test:test@localhost:5432/test")
os.environ.setdefault("GO_API", "http://localhost:8080")

import pandas as pd  # noqa: E402

from app.modules.analytics import service  # noqa: E402


def _fake_products(*ids):
    """构造 _resolve_category 期望的查询结果。"""
    return pd.DataFrame([{"id": i} for i in ids])


class FilterTests(unittest.TestCase):
    def test_without_any_filter_produces_no_where(self):
        where, params = service._filters()
        self.assertEqual(where, "")
        self.assertEqual(params, {})

    def test_category_adds_product_id_condition(self):
        with patch.object(service.pd, "read_sql", return_value=_fake_products(5)):
            where, params = service._filters(category="computer")

        self.assertIn("product_id = %(product_id)s", where)
        self.assertEqual(params["product_id"], 5)

    def test_unknown_category_raises(self):
        with patch.object(service.pd, "read_sql", return_value=pd.DataFrame(columns=["id"])):
            with self.assertRaises(service.CategoryNotFoundError):
                service._filters(category="__nope__")

    def test_category_condition_is_not_added_when_absent(self):
        """回归：曾经写成 `if product_id is None`，导致无品类时拼出 `product_id = NULL`。"""
        where, params = service._filters()
        self.assertNotIn("product_id", where)
        self.assertNotIn("product_id", params)

    def test_month_range_uses_month_column(self):
        where, params = service._filters(start_month="2024-01", end_month="2024-06")
        self.assertIn("month >= %(start_month)s", where)
        self.assertIn("month <= %(end_month)s", where)
        self.assertEqual(params["start_month"], "2024-01")
        self.assertEqual(params["end_month"], "2024-06")

    def test_empty_strings_are_ignored(self):
        """回归：曾经用 `is not None` 判断，空串也会拼进 WHERE。"""
        where, params = service._filters(productor="", start_month="", end_month="")
        self.assertEqual(where, "")
        self.assertEqual(params, {})

    def test_all_conditions_are_joined_with_and(self):
        with patch.object(service.pd, "read_sql", return_value=_fake_products(1)):
            where, params = service._filters(
                category="phone",
                productor="Apple",
                start_month="2024-01",
                end_month="2024-12",
            )

        self.assertTrue(where.startswith("WHERE "))
        self.assertEqual(where.count("AND"), 3)
        self.assertEqual(
            params,
            {
                "product_id": 1,
                "productor": "Apple",
                "start_month": "2024-01",
                "end_month": "2024-12",
            },
        )


class CategoryNotFoundTests(unittest.TestCase):
    def test_error_carries_category_name(self):
        exc = service.CategoryNotFoundError("comptuer")
        self.assertIn("comptuer", str(exc))


if __name__ == "__main__":
    unittest.main()
