# go_api
```bash
cd go_api
go run cmd/server/main.go
```
# python_code
``` bash  
cd python_code
uv run uvicorn app.main:app --reload --port 8000
```

---

# API 接口说明（Go 服务）

Base URL：`http://localhost:8080`

## 通用约定

**响应信封**（除 `/health` 外全部接口）：

```json
{ "code": 0, "msg": "ok", "data": {} }
```

**错误码**

| code | HTTP | 含义 |
|---|---|---|
| 0 | 200 | 成功 |
| 10001 | 400 | 参数错误 |
| 10002 | 404 | 资源不存在 |
| 10004 | 500 | 服务器内部错误 |
| 20001 | 502 | Python 分析服务不可用 |

**路径参数 `:id`**：无符号整数且必须大于 0，非数字或为 0 时返回 `400 / 10001`。

## 路由总览

| # | 方法 | 路径 | 参数位置 |
|---|---|---|---|
| 1 | GET | `/health` | 无 |
| 2 | GET | `/api/products` | 无 |
| 3 | POST | `/api/products` | JSON Body |
| 4 | POST | `/api/sales/batch` | JSON Body |
| 5 | POST | `/api/imports` | JSON Body |
| 6 | GET | `/api/imports/:id` | Path |
| 7 | GET | `/api/imports/:id/columns` | Path |
| 8 | POST | `/api/imports/:id/transform` | Path |
| 9 | GET | `/api/mappings` | Query |
| 10 | POST | `/api/mappings` | JSON Body |
| 11 | POST | `/api/summary/refresh` | 无 |
| 12 | GET | `/api/summary/verify` | 无 |
| 13 | GET | `/api/analytics/sales` | Query |
| 14 | GET | `/api/analytics/top-products` | Query |
| 15 | GET | `/api/analytics/brands` | Query |
| 16 | GET | `/api/analytics/monthly-trend` | Query |

---

## 1. 健康检查

### `GET /health`

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| — | — | — | 无参数 |

返回纯文本 `ok`（**不走 JSON 信封**）。

```bash
curl http://localhost:8080/health
```

---

## 2. 品类（products）

### `GET /api/products`

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| — | — | — | 无参数 |

返回 `products` 表全部记录，按 `id` 倒序。

```json
{ "id": 1, "name": "phone", "description": "", "price": 0,
  "created_at": "2026-10-21T10:00:00Z", "updated_at": "2026-10-21T10:00:00Z" }
```

### `POST /api/products`

**JSON Body**

| 字段 | 类型 | 必填 | 约束 | 说明 |
|---|---|---|---|---|
| `name` | string | 是 | 1–100 字符 | 品类名，如 `phone`、`computer`；全局唯一 |
| `price` | number | 否 | `>= 0`，默认 `0` | 品类参考价 |

> `description` 在 model 上存在，但**请求 DTO 未绑定该字段**，传入不会保存。

```bash
curl -X POST http://localhost:8080/api/products \
  -H 'Content-Type: application/json' \
  -d '{"name":"computer","price":0}'
```

失败场景：`name` 为空或超长 → `400 / 10001`；`name` 重复 → 唯一索引冲突 → `400 / 10001`。

---

## 3. 明细层（sales）

### `POST /api/sales/batch`

直接写入标准层，不经过原始层与映射层。**追加写入，非幂等**。

**JSON Body**

| 字段 | 类型 | 必填 | 约束 | 说明 |
|---|---|---|---|---|
| `product` | string | 是 | 1+ 字符 | 品类名，服务端按 `products.name` 解析成 `product_id`，不存在则报错 |
| `batch_id` | integer | 否 | — | 关联的批次 id，手工导入可留空 |
| `rows` | array | 是 | 至少 1 条 | 销售明细行 |

**`rows[]` 元素**

| 字段 | 类型 | 必填 | 约束 | 说明 |
|---|---|---|---|---|
| `productor` | string | 是 | 1–100 字符 | 品牌 |
| `name` | string | 是 | 1–100 字符 | 商品名 |
| `price` | number | 否 | `>= 0` | 单价 |
| `sales` | integer | 否 | `>= 0` | 销量 |
| `month` | string | 是 | `^\d{4}-(0[1-9]\|1[0-2])$` | 月份，如 `2024-01` |

**响应 `data`**

```json
{ "product_id": 2, "batch_id": null, "inserted": 400 }
```

```bash
curl -X POST http://localhost:8080/api/sales/batch \
  -H 'Content-Type: application/json' \
  -d '{"product":"phone","rows":[
        {"productor":"Apple","name":"iPhone 15","price":5999,"sales":100,"month":"2024-01"}
      ]}'
```

---

## 4. 原始层（imports / raw）

### `POST /api/imports`

写入一批原始数据（JSONB），不校验业务字段。

**JSON Body**

| 字段 | 类型 | 必填 | 约束 | 说明 |
|---|---|---|---|---|
| `source` | string | 是 | 1–255 字符 | 来源文件名或标识 |
| `category` | string | 是 | 1–50 字符 | 品类名 |
| `rows` | array\<object\> | 是 | 1–10000 条 | 每行为任意键值对象，**不能是空对象 `{}`** |

**响应 `data`**

```json
{ "batch_id": 2, "row_count": 400 }
```

```bash
curl -X POST http://localhost:8080/api/imports \
  -H 'Content-Type: application/json' \
  -d '{"source":"computer_sales.csv","category":"computer",
       "rows":[{"productor":"Apple","name":"MacBook Pro 14","price":"14999.00","sales":"114212","month":"2024-01"}]}'
```

### `GET /api/imports/:id`

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | Path | integer | 是 | 批次 id |

返回该批次已入库的原始行数，`data` **是一个数字**：

```json
{ "code": 0, "msg": "ok", "data": 400 }
```

批次不存在 → `404 / 10002`。

### `GET /api/imports/:id/columns`

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | Path | integer | 是 | 批次 id |

返回该批次出现过的所有列名及样例值，用于确认映射前源列名的实际写法。

**响应 `data`**

```json
[ { "column": "productor", "sample": "Apple" },
  { "column": "name",      "sample": "MacBook Pro 14" } ]
```

批次不存在 → `404 / 10002`。

### `POST /api/imports/:id/transform`

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | Path | integer | 是 | 批次 id |

把原始数据按 `field_mappings`（品类维度）翻译成标准字段，写入 `sales`。**幂等**：同批次重复调用只覆盖，不翻倍。

**响应 `data`**

| 字段 | 类型 | 说明 |
|---|---|---|
| `batch_id` | uint | 批次 id |
| `product` | string | 品类名 |
| `inserted` | int | 写入 `sales` 的行数 |
| `ignored_columns` | string[] | 未能映射到标准字段、且不是标准字段名的列 |
| `summary_refreshed` | bool | 是否已刷新汇总层 |
| `summary_error` | string | 刷新汇总层失败时的原因（为空时该字段不出现） |

批次不存在 → `404 / 10002`；转换失败（缺字段、歧义映射、月份格式错误等）→ `400 / 10001`。

> 转换完成后建议显式调用 `POST /api/summary/refresh` 重建汇总层。

---

## 5. 映射层（mappings）

### `POST /api/mappings`

配置一条「源列名 → 标准字段」规则。同一 `(category, source_column)` 重复提交会**覆盖** `target_field`。

**JSON Body**

| 字段 | 类型 | 必填 | 约束 | 说明 |
|---|---|---|---|---|
| `category` | string | 是 | 1–50 字符 | 品类名 |
| `source_column` | string | 是 | 1–100 字符 | 原始表里的列名 |
| `target_field` | string | 是 | 1–50 字符 | 目标标准字段，必须在白名单内：`productor` / `name` / `price` / `sales` / `month` |

```bash
curl -X POST http://localhost:8080/api/mappings \
  -H 'Content-Type: application/json' \
  -d '{"category":"computer","source_column":"厂商","target_field":"productor"}'
```

`target_field` 不在白名单 → `400 / 10001`。

### `GET /api/mappings`

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `category` | Query | string | 是 | 品类名；缺失返回 `400 / 10001`（"缺少 category 参数"） |

**响应 `data`**

```json
[ { "id": 1, "category": "computer", "source_column": "厂商",
    "target_field": "productor", "created_at": "...", "updated_at": "..." } ]
```

---

## 6. 汇总层（DWS）

### `POST /api/summary/refresh`

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| — | — | — | 无参数 |

在一个事务内 `TRUNCATE dws_sales` 后按 `(product_id, productor, name, month)` 全量重建。

**响应 `data`**

```json
{ "rows": 200 }
```

### `GET /api/summary/verify`

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| — | — | — | 无参数 |

对账：用双向 `EXCEPT` 比较 `sales` 与 `dws_sales` 在同一粒度上的 `total_sales`。

**响应 `data`**

```json
{ "ok": true, "diff_count": 0, "differences": [] }
```

`differences[]` 元素：`{ product_id, productor, name, month, total_sales, side }`，`side` 为 `detail_only`（明细有、汇总无）或 `summary_only`（汇总有、明细无）。

---

## 7. 分析层（analytics）

四个接口都是**代理**到 Python 服务（默认 `http://localhost:8000`），Go 侧只做参数透传与 RequestID 传递。

### 公共 Query 参数

| 参数 | 类型 | 适用接口 | 必填 | 说明 |
|---|---|---|---|---|
| `category` | string | 全部 4 个 | 否 | 品类名，如 `phone` / `computer`；不传表示全部品类；传了但品类不存在 → `404 / 10002` |
| `limit` | integer | `top-products`、`brands` | 否 | 返回条数上限，见各自说明；解析失败或 `<= 0` 时回落为 `5` |
| `start_month` | string | `sales` | 否 | 起始月份（含），`YYYY-MM` |
| `end_month` | string | `sales` | 否 | 结束月份（含），`YYYY-MM` |
| `productor` | string | `monthly-trend` | 否 | 品牌名，精确匹配 |

> 时间与品类条件均为「传了才生效」，不传即不加过滤条件。

### `GET /api/analytics/sales`

整体概览。

| 参数 | 必填 | 说明 |
|---|---|---|
| `start_month` | 否 | 起始月份，`YYYY-MM` |
| `end_month` | 否 | 结束月份，`YYYY-MM` |
| `category` | 否 | 品类过滤 |

**响应 `data`**

```json
{ "category": "computer", "start_month": "2024-01", "end_month": "2024-06",
  "brand_count": 3, "product_count": 8,
  "total_sales": 1234567, "total_amount": 9876543.21, "avg_price": 5999.5 }
```

`avg_price` 为按记录数加权的平均单价（`SUM(price_sum) / SUM(record_count)`）。

### `GET /api/analytics/top-products`

| 参数 | 必填 | 约束 | 说明 |
|---|---|---|---|
| `limit` | 否 | 1–500，默认 5 | 返回条数 |
| `category` | 否 | — | 品类过滤 |

**响应 `data`**

```json
[ { "brand": "Apple", "name": "MacBook Pro 14",
    "total_sales": 1000000, "total_amount": 14999000000, "avg_price": 14999 } ]
```

按 `total_sales` 倒序。

### `GET /api/analytics/brands`

| 参数 | 必填 | 约束 | 说明 |
|---|---|---|---|
| `limit` | 否 | 1–50，默认 5 | 返回条数 |
| `category` | 否 | — | 品类过滤 |

**响应 `data`**

```json
[ { "brand": "Apple", "total_sales": 2000000,
    "total_amount": 29998000000, "product_count": 4 } ]
```

> **注意**：不带 `category` 时，同一品牌在不同品类下的数据会被合并（例如 `Apple` 同时包含手机与电脑）。需要分类看时必须显式传 `category`。

### `GET /api/analytics/monthly-trend`

| 参数 | 必填 | 说明 |
|---|---|---|
| `productor` | 否 | 品牌名，精确匹配 |
| `category` | 否 | 品类过滤 |

**响应 `data`**

```json
[ { "month": "2024-01", "total_sales": 500000, "total_amount": 7500000000 } ]
```

按 `month` 升序。

### 示例

```bash
# 全部品类
curl 'http://localhost:8080/api/analytics/sales?start_month=2024-01&end_month=2024-06'

# 指定品类
curl 'http://localhost:8080/api/analytics/brands?category=computer&limit=10'

# 品类 + 品牌
curl 'http://localhost:8080/api/analytics/monthly-trend?category=phone&productor=Apple'

# 未知品类
curl -i 'http://localhost:8080/api/analytics/brands?category=comptuer'
# → HTTP 404, {"code":10002,...}
```

> `limit` 超出上限（如 `top-products?limit=1000`）会被 Python 侧的参数校验拒绝（HTTP 422），Go 侧统一按上游错误返回 `502 / 20001`。

---

## 附：Python 服务自身的路由

以下接口注册在 Python 服务（`:8000`）上，Go 未注册同名路由：

| 方法 | 路径 | 参数 | 说明 |
|---|---|---|---|
| POST | `/import/upload` | form-data：`file`（必填，`.csv`/`.xls`/`.xlsx`）、`category`（必填） | 上传表格，解析后转调 Go 的 `POST /api/imports` |
| GET | `/analytics/*` | 同上文分析层的 Query 参数 | Go 的分析接口即代理到这里 |
| GET | `/health` | 无 | 返回 `{"status":"ok"}` |
