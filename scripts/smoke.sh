#!/usr/bin/env bash
set -euo pipefail

GO=http://localhost:8080
PY=http://localhost:8000

fail() { echo "FAIL: $1"; exit 1; }

# 1. 健康检查
curl -fsS "$GO/health" > /dev/null || fail "go health"
curl -fsS "$PY/health" > /dev/null || fail "python health"

# 2. 品类必须存在
curl -fsS "$GO/api/products" | grep -q '"phone"' || fail "products"

# 3. 汇总层与明细层对账
ok=$(curl -fsS "$GO/api/summary/verify" | python -c 'import sys,json;print(json.load(sys.stdin)["data"]["ok"])')
[ "$ok" = "True" ] || fail "summary verify diff"

# 4. 品类过滤必须真的生效（回归 is None 那个 bug）
a=$(curl -fsS "$GO/api/analytics/brands?category=phone" | python -c 'import sys,json;print(len(json.load(sys.stdin)["data"]))')
[ "$a" -gt 0 ] || fail "category filter returned empty"

# 5. 不带品类必须非空（回归 product_id = NULL 那个 bug）
b=$(curl -fsS "$GO/api/analytics/monthly-trend" | python -c 'import sys,json;print(len(json.load(sys.stdin)["data"]))')
[ "$b" -gt 0 ] || fail "unfiltered query returned empty"

echo "smoke OK"