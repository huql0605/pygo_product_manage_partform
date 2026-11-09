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

## 首次启动

```bash
# 1. 准备环境变量（示例文件不含真实密码）
cp go_api/.env.example go_api/.env
cp python_code/.env.example python_code/.env
# 编辑两个 .env，填入实际的数据库连接串

# 2. 建库（只需一次）
createdb db1

# 3. 启动 Go，AutoMigrate 会自动建表（含 raw_files、summary_refresh_tasks）并补齐 attrs 的 GIN 索引
cd go_api && go run ./cmd/server

# 4. 另开终端启动 Python
cd python_code && uv run uvicorn app.main:app --reload --port 8000

# 5. 冒烟验证：服务在线、数据非空、汇总一致、品类过滤生效
bash scripts/smoke.sh
```

## 测试

```bash
# Go：无数据库时自动跳过集成测试
cd go_api && go test ./...

# Go 集成测试（需要一个库名含 test 的独立数据库，避免误删开发数据）
# 凭据请从 .env 或环境变量读取，不要写进文档
TEST_DATABASE_URL='host=localhost user=<db_user> password=<db_password> dbname=db1_test port=5432 sslmode=disable' \
  go test ./internal/summary -v

# Python
cd python_code && python -m unittest discover -s tests -v
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
| 5 | GET | `/api/imports` | Query |
| 6 | POST | `/api/imports` | JSON Body |
| 7 | GET | `/api/imports/:id` | Path |
| 8 | GET | `/api/imports/:id/columns` | Path |
| 9 | POST | `/api/imports/:id/transform` | Path |
| 10 | DELETE | `/api/imports/:id` | Path |
| 11 | GET | `/api/mappings` | Query |
| 12 | POST | `/api/mappings` | JSON Body |
| 13 | POST | `/api/summary/refresh/:id` | Path |
| 14 | POST | `/api/summary/refresh-all` | 无 |
| 15 | GET | `/api/summary/verify` | 无 |
| 16 | GET | `/api/analytics/sales` | Query |
| 17 | GET | `/api/analytics/top-products` | Query |
| 18 | GET | `/api/analytics/brands` | Query |
| 19 | GET | `/api/analytics/monthly-trend` | Query |
| 20 | GET | `/api/analytics/attributes` | Query |
| 21 | POST | `/api/import-uploads` | Multipart |

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

原始层由两张表组成：

- `raw_files`：原始文件字节（`bytea`）与 SHA-256 指纹，用于重放和去重。
- `raw_records`：解析后的行快照（JSONB），用于映射转换。

文件上传有三种入口：

| 入口 | 谁调用 | 说明 |
|---|---|---|
| `POST :8000/import/upload` | 普通用户 / 前端 | Python 读取文件、计算指纹、解析 rows，再转调 Go |
| `POST :8080/api/import-uploads` | 已解析好 rows 的内部调用方 | 一次请求同时提交文件字节与 rows，属于内部接口，不建议直接暴露到公网 |
| `POST :8080/api/imports` | 旧调用方 / 纯 JSON 场景 | 只提交 rows，不保存原始文件，不参与指纹去重 |

### `POST /api/import-uploads`

上传原始文件并写入批次。请求类型为 `multipart/form-data`。

| 字段 | 类型 | 必填 | 约束 | 说明 |
|---|---|---|---|---|
| `file` | 文件 | 是 | 最大 32MB | 原始文件字节 |
| `category` | string | 是 | 1–50 字符 | 品类名 |
| `source` | string | 否 | 1–255 字符 | 缺省使用文件名 |
| `reader_version` | string | 否 | 最多 32 字符 | 解析器版本，如 `csv-v1`、`excel-v1` |
| `file_hash` | string | 否 | 64 位十六进制 | 客户端计算的 SHA-256；服务端会重新计算并比对 |
| `rows` | string | 是 | 1–10000 行 | 已解析行的 JSON 数组字符串 |

服务端会重新计算文件 SHA-256，并执行去重：

- 同一 `(category, file_hash)` 已存在批次时，返回已有批次，`duplicated` 为 `true`，不会重复写 `raw_records`。
- 同一文件用于不同品类时，复用同一条 `raw_files`，新建一个批次。
- 不提供 `file_hash` 时按文件内容计算，仍然参与去重。

**响应 `data`**

```json
{ "batch_id": 13, "row_count": 168, "duplicated": false,
  "file_hash": "bc9098f6d1d29c61a1d0ed338685306e78abbe14d032393bc1e0ab97cb7606da" }
```

重复上传时：

```json
{ "batch_id": 13, "row_count": 168, "duplicated": true,
  "file_hash": "bc9098f6d1d29c61a1d0ed338685306e78abbe14d032393bc1e0ab97cb7606da" }
```

```bash
# 直接调用 Go 时需要自己提供 rows
curl -X POST http://localhost:8080/api/import-uploads \
  -F 'file=@phone_sales.csv;type=text/csv' \
  -F 'category=phone' \
  -F 'reader_version=csv-v1' \
  -F 'rows=<rows.json'
```

> 普通上传建议走 Python 的 `/import/upload`：它会完成 CSV/Excel 解析并自动提交 rows；直接调用 Go 的 `/api/import-uploads` 时，`rows` 必须由调用方保证与文件内容一致。

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
{ "batch_id": 2, "row_count": 400, "duplicated": false }
```

> 该入口不保存原始文件，`file_hash` 为空，不参与文件指纹去重。

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
| `summary_pending` | bool | 是否已写入异步刷新任务 |
| `summary_refreshed` | bool | 兼容字段，异步模式下恒为 `false` |
| `summary_error` | string | 保留字段，异步模式下不出现 |

批次不存在 → `404 / 10002`；转换失败（缺字段、歧义映射、月份格式错误等）→ `400 / 10001`。

> `transform` 在同一事务内把汇总刷新任务写入 `summary_refresh_tasks`，由后台 worker 异步重算受影响 grain。接口返回时 `dws_sales` 可能短暂滞后；需要立即全量修复时调用 `POST /api/summary/refresh-all`。

### `GET /api/imports`

分页列出批次。

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `category` | Query | string | 否 | 按品类过滤 |
| `status` | Query | string | 否 | `loaded` / `mapped`；`failed` 仅可能出现在旧数据中 |
| `page` | Query | integer | 否 | 页码，从 1 开始，默认 1 |
| `size` | Query | integer | 否 | 每页条数，默认 20，上限 200 |

**响应 `data`**

```json
{ "items": [ { "id": 9, "source": "computer_sales.csv", "category": "computer",
               "status": "mapped", "row_count": 120,
               "file_hash": "bc9098...",
               "raw_file_id": 3,
               "reader_version": "csv-v1",
               "created_at": "..." } ],
  "total": 9, "page": 1, "size": 20 }
```

旧批次没有原始文件，`file_hash` 为空串，`raw_file_id` 为 `null`，`reader_version` 为空。

### `DELETE /api/imports/:id`

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | Path | integer | 是 | 批次 id |

在同一个事务内删除该批次的 `sales` 明细行、`raw_records` 与 `import_batches` 记录，并写入汇总刷新任务。删除前会先保存该批次覆盖的 grain，worker 据此重算或删除汇总行。`raw_files` 中的原始文件不会随批次删除，保留用于审计和后续重放。**不可撤销**，重跑需要重新上传或从原始文件重放。

**响应 `data`**

```json
{ "batch_id": 9, "deleted_sales": 120, "summary_pending": true }
```

批次不存在 → `404 / 10002`。刷新由后台 worker 完成，接口返回后 `dws_sales` 可能短暂滞后；需要立即全量修复时调用 `POST /api/summary/refresh-all`。

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

### `POST /api/summary/refresh/:id`

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `id` | Path | integer | 是 | 批次 id |

按该批次当前明细覆盖的 grain 同步重算 `dws_sales`。适合手工修复单个批次，不影响其他 grain。

### `POST /api/summary/refresh-all`

无参数。在一个事务内 `DELETE FROM dws_sales` 后按 `(product_id, productor, name, month)` 全量重建，作为兜底入口。

**响应 `data`**

```json
{ "rows": 200 }
```

### 汇总刷新任务

`transform` 和 `DELETE /api/imports/:id` 会在写明细的同一事务里向 `summary_refresh_tasks` 写入任务，后台 worker 每秒领取并重算。任务状态为 `pending` / `done` / `failed`，失败按 `attempts` 退避重试，超过 10 次标记为 `failed`。接口返回 `summary_pending=true` 只表示任务已入队，`dws_sales` 允许短暂滞后。

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
| `start_month` | string | 全部 4 个 | 否 | 起始月份（含），`YYYY-MM` |
| `end_month` | string | 全部 4 个 | 否 | 结束月份（含），`YYYY-MM` |
| `productor` | string | 全部 4 个 | 否 | 品牌名，精确匹配 |

> 时间、品类、品牌三个条件在 4 个接口上行为一致：传了才生效，不传即不加过滤条件。

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
  "total_sales": 1234567, "total_amount": 9876543.21,
  "avg_listed_price": 5999.5, "avg_selling_price": 5888.8,
  "avg_price": 5999.5 }
```

- `avg_listed_price`：按记录数加权的挂牌均价（`SUM(price_sum) / SUM(record_count)`）。
- `avg_selling_price`：按销量加权的成交均价（`SUM(total_amount) / SUM(total_sales)`）。
- `avg_price`：兼容旧调用方，值等于 `avg_listed_price`。

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

### `GET /api/analytics/attributes`

按某个属性分组统计销量与金额。数据源是 `sales` 明细而不是 `dws_sales`，属于**探索口径**，与上面 4 个接口的确认口径区分。

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `key` | Query | string | 是 | 属性名，必须命中白名单 `ALLOWED_ATTRS` |
| `limit` | Query | integer | 否 | 默认 20，上限 100 |
| `category` | Query | string | 否 | 品类过滤 |
| `start_month` | Query | string | 否 | 起始月份，`YYYY-MM` |
| `end_month` | Query | string | 否 | 结束月份，`YYYY-MM` |

**响应 `data`**

```json
{ "key": "storage", "category": "computer", "start_month": null, "end_month": null,
  "items": [ { "value": "512G", "total_sales": 123456,
               "total_amount": 987654321.0, "record_count": 24 } ] }
```

**白名单**：由 `python_code/.env` 的 `ALLOWED_ATTRS` 控制，默认 `storage,screen,cpu,ram,color,weight`；设为 `*` 时放行任意合法命名的 key。未登记的 key 返回 `400 / 10001`。

```bash
curl 'http://localhost:8080/api/analytics/attributes?key=storage&category=computer'
```

> 属性以字符串形式存储在 `sales.attrs` 中（例如 `{"storage": "512G"}`），数值型属性需要在 SQL 里显式转型，例如 `SUM((attrs->>'weight')::numeric)`。

---

## 附：Python 服务自身的路由

以下接口注册在 Python 服务（`:8000`）上，Go 未注册同名路由：

| 方法 | 路径 | 参数 | 说明 |
|---|---|---|---|
| POST | `/import/upload` | form-data：`file`（必填，`.csv`/`.xls`/`.xlsx`，默认上限 32MB）、`category`（必填） | 读取文件、计算 SHA-256、解析 rows，然后转调 Go 的 `POST /api/import-uploads` |
| GET | `/analytics/*` | 同上文分析层的 Query 参数 | Go 的分析接口即代理到这里 |
| GET | `/health` | 无 | 返回 `{"status":"ok"}` |

Python 侧通过环境变量 `MAX_UPLOAD_BYTES` 控制上传上限，默认 `33554432`（32MB）。Python 上传响应比旧版本多两个字段：

```json
{ "code": 0, "msg": "ok",
  "data": { "batch_id": 13, "row_count": 168, "category": "phone",
            "source": "phone_sales.csv", "duplicated": false,
            "file_hash": "bc9098f6d1d29c61a1d0ed338685306e78abbe14d032393bc1e0ab97cb7606da" } }
```
