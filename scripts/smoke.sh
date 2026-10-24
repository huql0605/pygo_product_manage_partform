#!/usr/bin/env bash
# 一键冒烟：验证两个服务在线、数据链路一致、分析接口的筛选确实生效。
# 用法：bash scripts/smoke.sh
set -uo pipefail

GO="${GO:-http://localhost:8080}"
PY="${PY:-http://localhost:8000}"
PYBIN="${PYBIN:-python3}"

pass() { echo "  ok   $1"; }
fail() {
  echo "  FAIL $1" >&2
  [ $# -gt 1 ] && echo "       $2" >&2
  exit 1
}

# 读 stdin 的 JSON，输出 data 数组长度；顺便保证失败时能把原始响应打出来
len_of_data() {
  "$PYBIN" -c 'import sys,json;d=json.load(sys.stdin).get("data") or [];print(len(d))'
}

echo "[1/6] 健康检查"
curl -fsS --max-time 5 "$GO/health" >/dev/null || fail "go /health" "$GO/health 不可达"
curl -fsS --max-time 5 "$PY/health" >/dev/null || fail "python /health" "$PY/health 不可达，先确认 uvicorn 已启动"
pass "两个服务在线"

echo "[2/6] 品类存在"
resp=$(curl -sS --max-time 5 "$GO/api/products") || fail "go /api/products" "请求失败"
printf '%s' "$resp" | grep -q '"phone"'    || fail "products 缺少 phone"    "$resp"
printf '%s' "$resp" | grep -q '"computer"' || fail "products 缺少 computer" "$resp"
pass "phone 与 computer 均已创建"

echo "[3/6] 明细数据非空"
total=$(curl -sS --max-time 10 "$GO/api/analytics/sales" \
  | "$PYBIN" -c 'import sys,json;print(json.load(sys.stdin)["data"]["total_sales"])') \
  || fail "analytics/sales" "解析失败，响应可能不是预期结构"
case "$total" in
  ''|*[!0-9]*) fail "analytics/sales" "total_sales 不是整数: $total" ;;
esac
[ "$total" -gt 0 ] || fail "sales 为空" "total_sales=0，说明 sales 表没有数据，先执行导入"
pass "total_sales=$total"

echo "[4/6] 汇总层与明细层对账"
ok=$(curl -sS --max-time 10 "$GO/api/summary/verify" \
  | "$PYBIN" -c 'import sys,json;print(json.load(sys.stdin)["data"]["ok"])') \
  || fail "summary/verify" "解析失败"
[ "$ok" = "True" ] || fail "汇总层与明细层不一致" "调用 GET /api/summary/verify 查看 differences"
pass "diff_count=0"

echo "[5/6] 品类过滤生效"
resp=$(curl -sS --max-time 10 "$GO/api/analytics/brands?category=phone") \
  || fail "brands?category=phone" "请求失败"
n=$(printf '%s' "$resp" | len_of_data) \
  || fail "brands?category=phone" "响应不是预期 JSON: $resp"
[ "$n" -gt 0 ] || fail "brands?category=phone 返回空" "$resp"
pass "phone 品牌数=$n"

echo "[6/6] 未知品类必须 404"
code=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 10 \
  "$GO/api/analytics/brands?category=__nope__")
[ "$code" = "404" ] || fail "未知品类未返回 404" "实际状态码 $code"
pass "未知品类返回 404"

echo
echo "smoke OK"