# 设计评审：分层数据平台（2026-10-31 复核更新）

> 基线：`4df6303`（2026-10-28，提交信息为“大改动，修改原始层没有存到原始数据的问题”）。
> 上一版评审写于 2026-10-21。此后代码已修掉一部分 P0，但原始层、手工数据生命周期、汇总层刷新/对账等结构性问题仍在。
> 本文保留原编号以便追溯，并新增 23–30 号问题；每条结论都对应仓库中的实际代码位置。

## 0. 本轮复核说明

**复核范围**：`README.md`、`docs/architecture.md`、`docs/design-review.md`、`go_api/`、`python_code/`（不含 `.venv`）。

**复核方法**：

1. 逐项对照上一版评审与当前代码，确认「声称已改」是否真的落地。
2. 重新阅读 README 与架构文档，找出文档与实现的偏差。
3. 运行现有测试，确认基线可用。

**本轮执行结果**：

| 检查 | 结果 |
|---|---|
| `cd go_api && go build ./...` | 通过 |
| `cd go_api && go test ./...` | 通过；`mapping` 单测通过，`summary` 的 3 个集成测试因未设置 `TEST_DATABASE_URL` 跳过 |
| `cd go_api && go vet ./...` | 通过 |
| `cd python_code && uv run python -m unittest discover -s tests -v` | 8 个用例全部通过 |
| 真实 PostgreSQL + 双服务联调 | **未执行**；未设置 `TEST_DATABASE_URL`，数据库相关结论来自代码审阅 |

**总体结论**：

- 上一轮最紧迫的金额精度（#2）、`avg_price` 口径（#3）、`product_count`（#4）、失败状态与数据矛盾（#5）、`attrs` 内容漂移（#6）已经修复，属于实质性改进。
- 但 #1（原始层不可重放）、#7（手工数据无法删除）两个 P0 仍在；#8/#9/#11/#12 等规模问题没有动。
- 本轮新增 3 个 P1 级问题，其中 **transform 对 `sales` 的解析会静默截断/接受负数和 NaN** 最值得优先修，它直接污染明细与汇总。
- 文档仍停留在修复前的语义（`failed` 状态、只返回 `avg_price`、grain 能支撑全部报表），需要与代码同步。

等级定义不变：

- **P0**：数据错误、数据丢失或不可回溯，必须改。
- **P1**：数据量、并发或脏输入上来后会产生错误结果，需要改。
- **P2**：口径不一致、维护成本高、错误语义混乱，建议改。

状态标记：**✅ 已修复** · **◐ 部分修复** · **❌ 仍存在** · **🆕 本轮新增**

---

## 1. 缺陷总览

| 编号 | 等级 | 问题 | 状态 | 位置 |
|---|---|---|---|---|
| 1 | P0 | 「原始层不解析」不成立，解析在 Python，且未保存原始文件 | ❌ | `python_code/app/modules/importer/csv_reader.py` |
| 2 | P0 | 金额用 `float64` / `float8` 存储 | ✅ | `go_api/internal/sale/model.go`、`summary/model.go` |
| 3 | P0 | `avg_price` 按记录数加权，缺「平均成交价」 | ✅ | `python_code/app/modules/analytics/service.py` |
| 4 | P0 | `product_count` 用 `COUNT(DISTINCT name)` | ✅ | `python_code/app/modules/analytics/service.py` |
| 5 | P0 | 转换失败后 `failed` 状态与上一轮 `sales` 数据矛盾 | ◐ | `go_api/internal/mapping/service.go` |
| 6 | P0 | `attrs` 只装未映射列，改映射后属性数据消失 | ✅ | `go_api/internal/mapping/service.go` |
| 7 | P0 | 手工写入（`batch_id` 为空）无删除路径 | ❌ | `go_api/internal/sale/service.go`、`raw/repository.go` |
| 8 | P1 | 汇总层全量 `TRUNCATE` 重建同步跑在请求里 | ❌ | `go_api/internal/summary/repository.go` |
| 9 | P1 | 对账只比 `total_sales`，无分页与上限 | ❌ | `go_api/internal/summary/repository.go` |
| 10 | P1 | 文档称 grain 能支撑全部报表，但 `attributes` 依赖 `attrs` | ❌ | `docs/architecture.md` 6.1 |
| 11 | P1 | 明细口径与汇总口径并存，响应不区分来源 | ❌ | `go_api/internal/analytics/handler.go`、Python service |
| 12 | P1 | `transform` 一次性全量载入内存，无流式分批 | ❌ | `go_api/internal/mapping/service.go` |
| 13 | P2 | 上游 4xx 被统一压成 502 | ❌ | `go_api/internal/analytics/client.go` |
| 14 | P2 | `limit` 在分析接口上处理规则不一致 | ❌ | `go_api/internal/analytics/handler.go` |
| 15 | P2 | `attributes` 的 `productor` 参数被静默丢弃 | ◐ | `python_code/app/api/analytics.py` |
| 16 | P2 | `start_month` / `end_month` 全链路无格式校验 | ❌ | `go_api/internal/analytics/handler.go`、Python service |
| 17 | P2 | 数据库层无外键、无 `CHECK`、状态无枚举 | ❌ | `go_api/internal/sale/model.go`、`raw/model.go` |
| 18 | P2 | `PurgeBatch` 直接删 `sales`；新增的 `uow` 未被使用 | ❌ | `go_api/internal/raw/repository.go`、`common/uow/uow.go` |
| 19 | P2 | 没有真正的 ADS 层，报表每次现算 | ❌ | `docs/architecture.md` 第 1 节 |
| 20 | P2 | Python + Pandas 承担的主要是纯 SQL 代理 | ❌ | 整体架构 |
| 21 | P2 | 测试偏纯函数，handler / 事务 / 并发无覆盖 | ◐ | `go_api/internal/*/`、`python_code/tests/` |
| 22 | P2 | 注释与实现不符（样例值、行数口径、路由注释） | ◐ | `raw/repository.go`、`router.go`、`raw/handler.go` |
| 23 | P1 | `transform` 用 `ParseFloat` + `int64` 解析销量，静默截断/接受负数/NaN | 🆕 | `go_api/internal/mapping/transform.go` |
| 24 | P1 | `POST /api/sales/batch` 的 `batch_id` 未校验归属，可挂到任意批次并被级联删除 | 🆕 | `go_api/internal/sale/service.go` |
| 25 | P1 | 同一批次并发 `transform` 可产生重复明细 | 🆕 | `go_api/internal/mapping/service.go`、`sale/repository.go` |
| 26 | P2 | `attrs` 的 key 是原始列名，中文 key 无法查询，默认白名单也漏了实际列 | 🆕 | `go_api/internal/mapping/service.go`、`python_code/app/modules/analytics/service.py` |
| 27 | P2 | `/import/upload` 阻塞事件循环、无大小/行数预检、客户端错误返回 502 | 🆕 | `python_code/app/api/importer.py` |
| 28 | P2 | 错误码映射不一致：转换失败一律 400，手工写入失败一律 500 | 🆕 | `go_api/internal/mapping/handler.go`、`sale/handler.go` |
| 29 | P2 | README / architecture 与当前实现多处漂移 | 🆕 | `README.md`、`docs/architecture.md` |
| 30 | P2 | `POST /api/imports` 不校验品类是否存在，产生永远无法转换的孤儿批次 | 🆕 | `go_api/internal/raw/service.go` |

---

## 2. 本轮确认已修复

### 2.1 金额精度（原 #2）✅

`sales.price`、`dws_sales.total_amount / price_sum / min_price / max_price` 已改为 `decimal.Decimal` + PostgreSQL `numeric(18,4)`：

- `go_api/internal/sale/model.go:16`
- `go_api/internal/summary/model.go:17-22`
- `go_api/internal/mapping/transform.go:82` 使用 `decimal.NewFromString`
- 聚合 SQL 直接 `SUM(sales * price)`，由数据库按 numeric 计算

只在 Python 出参时 `round(float(...), 2)`，属于展示层转换，库内不再累计浮点误差。

> 残留（低优先级）：`products.price` 仍是 `float64`（`product/model.go:9`），但它只是品类参考价，不参与销售金额累计，暂不影响报表。

### 2.2 `avg_price` 拆分（原 #3）✅

`get_sales_summary` 已同时返回两个口径：

```sql
SUM(price_sum) / SUM(record_count)  AS avg_listed_price
SUM(total_amount) / SUM(total_sales) AS avg_selling_price
```

见 `python_code/app/modules/analytics/service.py:54-55,68-70`。旧的 `avg_price` 作为 `avg_listed_price` 的兼容别名保留。

> 残留：`top-products` 仍只返回 `avg_price`，语义仍是“按记录数加权的挂牌价”（`service.py:98`）。如果前端把它当成交价用，会继续误导；见 5.10。

### 2.3 `product_count`（原 #4）✅

`get_sales_summary` 已改为：

```sql
COUNT(DISTINCT (productor, name)) AS product_count
```

见 `python_code/app/modules/analytics/service.py:51`。品牌排行里的 `COUNT(DISTINCT name)` 在 `GROUP BY productor` 下是正确的，不需要改。

### 2.4 失败状态与数据矛盾（原 #5）◐

`mapping.Service.Transform` 不再在失败时写 `status=failed`：

- `Transform` 只在核心转换成功后调用 `UpdateStatus(..., mapped)`（`mapping/service.go:84-101`）。
- 失败发生在写库之前，上一轮 `sales` 保持不变，语义自洽。
- 单测 `TestTransform_MissingField_DoesNotTouchStatus`、`TestTransform_AmbiguousColumns_ReportsRealError` 已钉住“失败不改状态”。

**残留问题**：

1. `UpdateStatus` 在 `ReplaceBatch` 提交之后、独立事务中执行（`mapping/service.go:91`）。若状态更新失败，接口返回错误，但明细已经替换成功，客户端会误判。
2. `raw.StatusFailed`（`raw/model.go:11`）已成为死常量；架构文档的状态图仍写 `loaded --> failed`。

更彻底的做法仍是将“替换明细 + 更新状态”放进同一事务，或引入 `transforming` 中间态。

### 2.5 `attrs` 不再是映射配置的副产物（原 #6）✅

`mapping/service.go:143-154` 现在把**所有非标准列**写入 `attrs`，与是否配置映射无关：

```go
for col, v := range obj {
    if IsStandardField(col) {
        continue
    }
    attrs[col] = valueToString(v)
}
```

映射规则只决定标准字段取值，不再影响 `attrs` 的键集合。重复 `transform` 不会因为补映射而丢属性。

> 新的边界问题见 5.11（key 命名与白名单）。

### 2.6 注释修正（原 #22）◐

- `product/repository.go` 原先错误的“获取 sales 表所有商品”注释已删除。
- 仍未修正的见 5.10：`router.go:54` 的路由注释、`ListColumns` 的样例值语义、`GET /api/imports/:id` 的行数口径。

---

## 3. P0：仍存在的正确性与可回溯问题

### 3.1 原始层仍然不是原始层（原 #1）❌

`docs/architecture.md` 仍声称“原样存 JSONB，不解析”，但真正的解析发生在 Python：

```python
pd.read_csv(source, dtype=str, encoding="utf-8-sig", keep_default_na=False)
```

见 `python_code/app/modules/importer/csv_reader.py:8-16` 与 `excel_reader.py:3-6`。

这意味着：

- `utf-8-sig` 已去 BOM，`dtype=str` 已做类型归一，`keep_default_na=False` 已改写空值策略；Go 侧存的只是 Python 二次序列化后的字典。
- 原始文件没有落盘，没有 SHA-256，没有 reader 版本号。解析规则变更、编码判断错误、Excel 数值精度问题都无法对历史批次重放。
- 同一文件重复上传无法识别，只能靠人工判断。

本次提交信息虽然提到“修改原始层没有存到原始数据的问题”，实际改的是 `attrs`（#6），**没有**新增原始文件留存或文件指纹。

**建议改法**：

1. 新增 `raw_files(sha256 PK, filename, content bytea | object_key, size, created_at)`。
2. `import_batches` 增加 `file_hash`、`reader_version`；`(category, file_hash)` 重复上传返回已有批次。
3. 原始文件与解析产物分离：解析产物是可重算的派生结果，原始字节才是 ODS。
4. 修正架构文档对“原始层”的定义。

**涉及文件**：`python_code/app/api/importer.py`、`python_code/app/modules/importer/`、`go_api/internal/raw/`、`docs/architecture.md`

---

### 3.2 手工写入的数据仍无删除路径（原 #7）❌

`POST /api/sales/batch` 仍允许 `batch_id` 为空直接写 `sales`（`sale/dto.go:14-18`、`sale/service.go:60-73`）。而删除路径只有：

```sql
DELETE FROM sales WHERE batch_id = ?
```

见 `raw/repository.go:93-108`。`batch_id IS NULL` 的行永远不会被命中。

结合“追加写入、非幂等”的语义，写错一次就会永久污染报表和汇总层，只能手工进数据库清理。

**建议改法**：

- 为手工写入也自动创建一个 `source=manual` 的批次，让所有明细都挂在批次下；或
- 提供按批次/按 `(productor, name, month)` 的明细回滚接口；至少不要允许匿名 NULL 批次长期存在。

**涉及文件**：`go_api/internal/sale/service.go`、`go_api/internal/sale/handler.go`、`go_api/internal/raw/repository.go`

---

## 4. P1：仍存在的规模与正确性问题

### 4.1 汇总层全量重建跑在请求里（原 #8）❌

`summary.Repository.RefreshAll` 仍是单事务 `TRUNCATE + INSERT ... GROUP BY`：

- `summary/repository.go:13-48`
- `transform` 成功后同步调用（`mapping/service.go:94-99`）
- 删除批次后同步调用（`raw/service.go:125-131`）

三重问题未变：

| 问题 | 说明 |
|---|---|
| O(N²) | 导入 N 个批次 = 重建 N 次全表，总代价随累计数据量平方增长 |
| 读阻塞 | `TRUNCATE` 持 `ACCESS EXCLUSIVE` 锁到事务提交，期间分析查询被挡住 |
| 请求超时 | `transform`/`delete` 耗时随累计数据量线性增长，前端先超时而后端仍在跑 |

另外 `sales` 上没有 `(product_id, productor, name, month)` 复合索引，刷新本身也是全表扫 + 排序。

**建议改法**：

- 改为增量重算受影响的 grain，用 `INSERT ... ON CONFLICT (grain) DO UPDATE`，避免 `TRUNCATE` 的排他锁。
- 把刷新移出请求链路，改为异步任务/队列；`transform` 只返回“明细已写入，汇总刷新中”。
- 补 `sales(product_id, productor, name, month)` 复合索引。

**涉及文件**：`go_api/internal/summary/repository.go`、`mapping/service.go`、`raw/service.go`

---

### 4.2 对账接口不完整（原 #9）❌

`summary.Repository.Verify` 仍只比较 `total_sales`：

```sql
WITH detail AS (SELECT ..., SUM(sales) AS total_sales FROM sales GROUP BY ...)
```

见 `summary/repository.go:60-80`。

| 问题 | 说明 |
|---|---|
| 漏校验 | `total_amount`、`price_sum`、`record_count`、`min/max_price` 全部未核对 |
| 无上限 | 双向 `EXCEPT` 全表执行，无 `limit`、无分页，差异可能是巨量 JSON |
| 无性能约束 | 数据量大时接口超时 |
| 无时间范围 | 无法只核对某个批次或月份 |

**建议改法**：

- 对账覆盖全部可加度量与分子分母，逐字段比较。
- 增加 `limit`、`offset`，返回前 N 条差异 + 总数。
- 支持按 `batch_id` 或 `month` 局部对账。

**涉及文件**：`go_api/internal/summary/repository.go`、`service.go`、`handler.go`

---

### 4.3 grain 声明与报表需求不符（原 #10）❌

`docs/architecture.md:229` 仍写：

> 这是能支撑全部现有报表的最小维度组合。

但 `attributes` 报表依赖 `sales.attrs`，`attrs` 不在 grain 中，只能改扫明细。该论断不成立。

**建议改法**：把文档改成“grain 支撑结构化四类报表，属性类报表属于探索口径”；若要把属性报表也纳入确认口径，新增 `dws_sales_attrs(key, value, ...)` 辅助汇总表。

**涉及文件**：`docs/architecture.md` 第 6 节

---

### 4.4 双口径并存且无闸门（原 #11）❌

`/api/analytics/attributes` 读 `sales`，其余四个接口读 `dws_sales`，但：

- 响应体没有 `source: "dws" | "detail"`。
- 没有 `as_of`（汇总层最后刷新时间）。
- 路径没有显式区分“确认口径 / 探索口径”。

汇总滞后时，“总销量”与“按属性分组销量之和”对不上，系统不会提示。

**建议改法**：

- 路径显式区分，如 `/api/analytics/explore/attributes`。
- 响应增加 `source` 与 `as_of`。
- 在 `attributes` 响应中返回与 `sales` 同口径的 `total_sales`，便于调用方自查。

**涉及文件**：`go_api/internal/router/router.go`、`analytics/handler.go`、Python service

---

### 4.5 `transform` 全量载入内存（原 #12）❌

`mapping/service.go:117-156` 仍然：

1. `ListRecords` 一次读全部 `raw_records`；
2. 构造 `[]sale.SaleRow` 全量切片；
3. 最后 `CreateInBatches` 一次性写入。

峰值内存约为批次体积的 3 倍。`max=10000` 的行数上限只是掩盖问题。

**建议改法**：按 `row_index` 游标分页流式处理，边解析边分批写入；错误信息仍保留行号。

**涉及文件**：`go_api/internal/mapping/service.go`、`raw/repository.go`

---

### 4.6 销量解析会静默截断、接受负数与 NaN（新增 #23）🆕

`mapping/transform.go:86-88`：

```go
salesNum, err := strconv.ParseFloat(values["sales"], 64)
...
Sales: int64(salesNum),
```

`ParseFloat` 的语义远宽于“销量”：

| 输入 | 结果 |
|---|---|
| `"12.9"` | `12`，静默截断 |
| `"-5"` | `-5`，负销量进入明细与汇总 |
| `"NaN"` / `"Inf"` | `ParseFloat` 成功，转 `int64` 结果依赖实现（amd64 上通常得到极小值），污染汇总 |
| `"9007199254740993"` | 超过 `2^53`，精度丢失 |

HTTP 的 `POST /api/sales/batch` 有 `binding:"gte=0"`（`sale/dto.go:9`），但 `transform` 走的是自己的 `buildRow`，**没有** `sales >= 0` 校验，也没有整数校验。

**建议改法**：

- 用 `strconv.ParseInt(values["sales"], 10, 64)`，拒绝小数、科学计数法、NaN/Inf。
- 显式校验 `sales >= 0`。
- 补单测覆盖 `"12.9"`、`"-5"`、`"NaN"`、超大整数。

**涉及文件**：`go_api/internal/mapping/transform.go`、`transform_test.go`

---

### 4.7 手工写入的 `batch_id` 未校验归属（新增 #24）🆕

`POST /api/sales/batch` 允许客户端自带 `batch_id`（`sale/dto.go:16`）。`sale.Service.CreateBatch` 直接透传：

```go
func (s *Service) CreateBatch(product string, batchID *uint, rows []SaleRow) ...
```

见 `sale/service.go:60-73`。服务端没有校验：

- 该批次是否存在；
- 批次品类是否等于 `product`；
- 该批次是否属于当前调用方（当前无鉴权，至少应校验前两项）。

后果：

1. 手工数据可以挂到别人的批次下，`DELETE /api/imports/:id` 会把它们一起删掉。
2. 重新 `transform` 该批次时，`ReplaceBatch` 的 `DELETE WHERE batch_id=?` 也会删掉手工行，造成“数据莫名其妙消失”。

**建议改法**：手工接口不接受客户端 `batch_id`，由服务端创建 `source=manual` 批次；若必须保留，先校验批次存在且 `category` 一致。

**涉及文件**：`go_api/internal/sale/service.go`、`raw/repository.go`

---

### 4.8 同一批次并发 `transform` 可产生重复明细（新增 #25）🆕

`Transform` 的执行序列是：

1. 读取批次与映射；
2. `sale.Repository.ReplaceBatch`：事务内 `DELETE FROM sales WHERE batch_id=?`，再 `INSERT`。

见 `sale/repository.go:20-30`。`sales` 上没有 `(batch_id, row_index)` 唯一约束（表里甚至没有 `row_index`），两个并发请求可能都完成 DELETE，然后各自 INSERT，最终明细翻倍。并发“转换 + 删除批次”也可能留下孤儿行。

**建议改法**：

- 在 `transform` 事务开始时对 `import_batches` 行加 `SELECT ... FOR UPDATE`，串行化同一批次操作；或
- 使用 PostgreSQL advisory lock（按 batch_id）；或
- 给 `sales` 增加 `row_index` 并加 `(batch_id, row_index)` 唯一约束，配合 upsert。

**涉及文件**：`go_api/internal/mapping/service.go`、`sale/repository.go`、`raw/repository.go`

---

## 5. P2：一致性与可维护性

### 5.1 上游 4xx 被压成 502（原 #13）❌

`analytics/client.go:84-85` 仍把上游所有 `>=300` 统一变成 `python returned %d`，handler 再统一返回 `502 / 20001`。

实际后果：

| 场景 | 真实原因 | 用户看到 |
|---|---|---|
| `top-products?limit=1000` | Python 校验拒绝（422） | 分析服务不可用（502） |
| 未知品类 | Python 返回 404 | Go 已单独映射为 404（这部分正确） |
| 上传超过 10000 行 | Go/Python 拒绝（400） | Python 侧又包成 502（见 5.12） |

**建议改法**：4xx 映射回 400 并透传 `detail`；只在连接失败、超时、5xx 时返回 502。

**涉及文件**：`go_api/internal/analytics/client.go`

---

### 5.2 `limit` 处理规则不一致（原 #14）❌

| 接口 | Go 侧 | Python 侧 | 结果 |
|---|---|---|---|
| `top-products` | 只兜底 `<=0`，无上限（`handler.go:44-49`） | `ge=1, le=500` | 超限返回 502 |
| `brands` | 只兜底 `<=0`，无上限（`handler.go:62-66`） | `ge=1, le=50` | 超限返回 502 |
| `attributes` | 夹到 100（`handler.go:103-109`） | `ge=1, le=100` | 正常 |

**建议改法**：Go 侧统一做上下限收敛（或直接 400 并给出允许范围），不要再依赖上游 422。

**涉及文件**：`go_api/internal/analytics/handler.go`

---

### 5.3 `attributes` 的 `productor` 仍被静默丢弃（原 #15）◐

Go 侧 `filterFrom` 会透传 `productor`（`analytics/handler.go:20-27`），但 Python 的 `/attributes` 没有该形参（`python_code/app/api/analytics.py:60-72`），`get_attribute_breakdown` 也没有接收它（`service.py:156-162`）。FastAPI 会忽略未知 query 参数，调用方以为过滤生效，实际没有。

README 已不再声称 `attributes` 支持 `productor`，文档侧修了一半，代码侧仍是静默丢弃。

**建议改法**：要么在 Python 侧补 `productor` 并透传给 `_filters`，要么在 Go 侧明确拒绝该参数。

**涉及文件**：`python_code/app/api/analytics.py`、`python_code/app/modules/analytics/service.py`

---

### 5.4 时间参数全链路零校验（原 #16）❌

`start_month` / `end_month` 从 Go 到 Python 全程只当字符串用，没有正则校验。传 `2024-1` 不会报错，而 `month >= '2024-1'` 在字符串比较下会产生静默错误结果。写入侧有 `^\d{4}-(0[1-9]|1[0-2])$` 校验（`mapping/transform.go:15`、`sale/service.go:9`），读取侧没有，两侧不对称。

**建议改法**：在 Go handler 层统一校验，不合法返回 400；或改用 `date` 类型存储每月第一天，从类型上避免字符串比较。

**涉及文件**：`go_api/internal/analytics/handler.go`

---

### 5.5 数据库层零约束（原 #17）❌

| 缺失项 | 位置 | 后果 |
|---|---|---|
| `sales.product_id` 外键 | `sale/model.go:12` | 可产生指向不存在品类的孤儿行 |
| `sales.batch_id` 外键 | `sale/model.go:13` | 被删除批次的数据可能残留 |
| `month` 的 `CHECK` | `sale/model.go:18` | 非标准格式可入库 |
| `status` 枚举 | `raw/model.go:18` | `?status=随便` 返回空列表而非 400 |

`router.MustMigrate`（`router/router.go:75-92`）只做 `AutoMigrate` 和 GIN 索引，模型没有关联字段，不会自动建外键。

**建议改法**：

- 补外键（`ON DELETE CASCADE` 或 `RESTRICT`，按业务语义定）。
- 补 `CHECK (month ~ '^\d{4}-(0[1-9]|1[0-2])$')`。
- `status` 用 `CHECK IN (...)` 或 PG 枚举类型约束。
- handler 层显式校验 `status` 白名单。

**涉及文件**：`go_api/internal/sale/model.go`、`raw/model.go`、`router/router.go`、`raw/handler.go`

---

### 5.6 `PurgeBatch` 跨层删除，`uow` 仍是死代码（原 #18）❌

`raw.Repository.PurgeBatch` 仍直接执行 `DELETE FROM sales`（`raw/repository.go:93-108`），与架构文档“repository 独占数据访问 / 严格单向依赖”冲突。

本次提交新增了 `go_api/internal/common/uow/uow.go`，但全仓库没有任何 `import` 引用。它没有解决问题，只是增加了未使用代码。

另外 `mapping` 声称“消费方定义接口”，但 `SaleWriter` 的签名直接使用 `sale.SaleRow`、`RawReader` 使用 `raw.ImportBatch`，类型没有隔离。

**建议改法**：

- 加 `ON DELETE CASCADE` 外键，让删除由数据库保证，`PurgeBatch` 只删批次表；
- 或真正引入 Unit of Work，把跨 repository 的事务放到 service 层；
- 删除未使用的 `uow` 包，避免误导。

**涉及文件**：`go_api/internal/raw/repository.go`、`go_api/internal/common/uow/uow.go`、`go_api/internal/mapping/service.go`

---

### 5.7 没有真正的 ADS 层（原 #19）❌

`docs/architecture.md` 把 `analytics` 报表等同于 ADS，但它本质是查询服务，不是物化的应用层结果表。报表口径没有版本，无法复现历史报表；后续加缓存、快照、导出也缺少落点。

**建议改法**：短期把文档表述改成“ADS = 查询服务”；长期为高频报表建立结果表或物化视图，并记录 `as_of`。

**涉及文件**：`docs/architecture.md` 第 1 节

---

### 5.8 Python + Pandas 的收益需要重新确认（原 #20）❌

五个分析接口仍是纯 SQL 聚合，Pandas 的贡献主要是 `to_dict(orient="records")` 和 `iloc[0]`。成本没有变化：多一个部署单元、多一次 HTTP 往返、`sales`/`dws_sales` schema 两份维护、Python 挂掉全部分析 502、错误语义翻译、与 Go 抢同一个数据库连接。

**建议**：回到需求确认是否存在 ML、复杂透视、非结构化处理。若没有，考虑把分析直接放进 Go；若有，给 Python 明确边界（只做需要 Python 生态的分析），不要继续代理纯 SQL。

---

### 5.9 测试覆盖仍不足（原 #21）◐

| 模块 | 现有测试 |
|---|---|
| `go_api/internal/mapping` | `service_test.go`、`transform_test.go`（纯函数 / 假实现） |
| `go_api/internal/summary` | `service_test.go`（3 个集成测试，依赖 `TEST_DATABASE_URL`，缺失时跳过） |
| `python_code/tests` | `test_analytics_filters.py`（8 个用例） |
| handler 层 | 无 |
| 事务 / 幂等 / 并发 | 无 |
| 解析边界（负数、NaN、小数销量） | 无 |

本轮实际运行：Go 测试通过、Python 8 个用例通过；数据库集成测试因未设置环境变量被跳过。

**建议改法**：

- 补 handler 测试（参数校验、错误码映射、404/400/502）。
- 补 `buildRow` 边界测试（#23）。
- 补并发 `transform` 测试（#25）。
- 补“失败不改状态 / 重复转换幂等 / 汇总刷新失败仍可重试”的集成测试。
- 加 CI，至少跑 `go build ./... && go test ./... && python -m unittest`。

**涉及文件**：`go_api/internal/*/`、`python_code/tests/`

---

### 5.10 注释、响应字段与实现不符（原 #22）◐

| 位置 | 现状 |
|---|---|
| `go_api/internal/router/router.go:54` | 注释写“获取 sales 表所有商品”，实际读 `products` 表 |
| `go_api/internal/raw/repository.go:37` | `ListColumns` 用 `MIN(row_data->>key)` 作“样例值”，取到字典序最小值，不是真实首例 |
| `go_api/internal/raw/handler.go:55-71` | `GET /api/imports/:id` 返回 `COUNT(raw_records)`，批次表已有 `row_count`，两者口径未统一说明 |
| `python_code/app/modules/analytics/service.py:98` | `top-products` 的 `avg_price` 是挂牌价口径，未跟随 #3 拆分改名 |
| `go_api/internal/config/config.go:34` | `python.timeout` 配置存在，但 `analytics.NewClient` 硬编码 10s（`client.go:26-28`），配置无效 |

**建议改法**：修正注释；`ListColumns` 改为 `ORDER BY row_index` 取真实首行；统一行数口径；`top-products` 明确字段名（`avg_listed_price` / `avg_selling_price`）；让 client 使用配置超时。

---

### 5.11 `attrs` 的 key 命名与查询白名单不匹配（新增 #26）🆕

`attrs` 的 key 是**原始列名**（`mapping/service.go:143-149`），而属性查询侧：

```python
_KEY_RE = re.compile(r"^[A-Za-z0-9_]{1,40}$")
```

见 `analytics/service.py:124`，只允许 ASCII 字母、数字、下划线。

后果：

1. 源表列名是 `屏幕`、`内存`、`重量` 时，值会正常写进 `attrs`，但 `GET /api/analytics/attributes?key=屏幕` 永远 400；即使 `ALLOWED_ATTRS=*` 也不行，因为正则先拒绝了。
2. 默认白名单是 `storage,screen,cpu,ram,color,weight`（`config.py:14`），而 `computer_sales.csv` 实际有 `memory,storage,cpu`，其中 `memory` 不在默认白名单里，按默认配置无法查询内存分布。
3. `attrs` 的键集合没有登记接口，运维只能靠猜或翻数据库。

**建议改法**：

- 允许 Unicode key（限制长度 1–40，拒绝控制字符），或增加“源列名 → 属性别名”的注册表。
- 提供 `GET /api/analytics/attributes/keys` 列出当前可查询的 key。
- 默认 `ALLOWED_ATTRS` 与实际数据对齐，文档写明需要配置。

**涉及文件**：`go_api/internal/mapping/service.go`、`python_code/app/modules/analytics/service.py`、`python_code/app/config.py`、`README.md`

---

### 5.12 上传接口的阻塞、限额与错误语义（新增 #27）🆕

`python_code/app/api/importer.py:18-52`：

1. 路由声明为 `async def`，但内部直接调用阻塞的 `pandas.read_csv` 和 `requests.post`，会卡住整个事件循环。
2. 先把整个文件读进 pandas，再交给 Go 的 `max=10000` 行校验；文件大小和行数在 Python 侧没有预检，大文件可先打爆内存。
3. `reader().parse(file.file)` 的解析异常没有捕获，坏 CSV 会变成 500。
4. `requests.raise_for_status()` 把 Go 的 400（空行、超过 10000 行等客户端错误）包成 `ImporterFailedError`，最终返回 **502**，与 #13 是同一类错误语义问题。

**建议改法**：

- 把同步解析/转发放到线程池（`def` 端点或 `run_in_threadpool`），或改用异步 HTTP 客户端。
- 限制上传字节数与最大行数，流式读取，超限直接 400。
- 捕获解析异常并返回 400。
- 区分上游 4xx/5xx，4xx 透传为 400 并带 `detail`。

**涉及文件**：`python_code/app/api/importer.py`、`python_code/app/modules/importer/`

---

### 5.13 handler 错误码映射仍不统一（新增 #28）🆕

| 入口 | 行为 | 问题 |
|---|---|---|
| `mapping/handler.go:56-63` | 只要不是 `ErrBatchNotFound`，一律 `400 / 10001` | 数据库错误、`UpdateStatus` 失败也被报成“参数错误” |
| `sale/handler.go:23-27` | 所有错误一律 `500 / 10004` | 品类不存在、价格为负等客户端错误被报成服务器故障 |
| `raw/handler.go:29-33` | 创建批次的持久化/序列化失败一律 500 | 没有区分可预期的参数错误与真正的内部错误 |

**建议改法**：定义 sentinel/typed error（`ErrInvalidParam`、`ErrNotFound`、`ErrConflict`、`ErrInternal`），handler 用 `errors.Is` 映射；service 不再用字符串表达错误类别。

**涉及文件**：`go_api/internal/common/errors/`、各 `handler.go`、各 `service.go`

---

### 5.14 README / architecture 与实现漂移（新增 #29）🆕

| 文档位置 | 文档说法 | 实际 |
|---|---|---|
| `README.md:414-417` | `sales` 只返回 `avg_price` | 代码返回 `avg_listed_price`、`avg_selling_price`、`avg_price` 三个字段 |
| `README.md:280` | `status` 支持 `loaded / mapped / failed` | 代码从不写 `failed`（`StatusFailed` 是死常量） |
| `README.md:271` | “转换完成后建议显式调用 refresh” | `transform` 已自动刷新，重复调用只会多一次全量重建 |
| `docs/architecture.md:431` | `loaded --> failed: 转换失败` | 代码失败时保持原状态，不写 `failed` |
| `docs/architecture.md:229` | grain 能支撑全部现有报表 | `attributes` 扫 `sales`，见 #10 |
| `docs/architecture.md:27` | 原始层“原样存 JSONB，不解析” | Python 已解析，见 #1 |
| `docs/architecture.md:353-366` | repository 独占数据访问、严格单向 | `PurgeBatch` 直写 `sales`，见 #18 |
| `.github/copilot-instructions.md:27,36` | “currently no test files” | 已有 Go/Python 测试 |

**建议改法**：以本文第 2 节为准同步 README 与 architecture；把“失败状态”从状态图和 README 参数表中删除，或先实现真正的 `failed` 语义；文档中的能力声明必须能被代码或测试验证。

---

### 5.15 原始批次不校验品类（新增 #30）🆕

`raw.Service.CreateBatch`（`raw/service.go:38-62`）只做 JSON 序列化与行数统计，不检查 `category` 是否存在于 `products`。

后果：

- 拼错品类（如 `comptuer`）会成功创建批次，直到 `transform` 才因为 `FindIDByName` 失败而报 400。
- 这类批次会留在列表里，只能手工删除；如果前端按“导入成功”提示，用户会误以为数据已入库。

**建议改法**：在原始层创建批次前校验品类存在（注入 `ProductResolver` 或由 service 调用 product repo），不存在返回 400；或明确设计为“原始层允许未知品类，转换时才校验”，并在 README 与响应中写清楚。

**涉及文件**：`go_api/internal/raw/service.go`、`raw/handler.go`

---

## 6. 修改路线（按投入产出排序）

### 第一批：低风险，先止血

- [ ] #23 销量解析：`ParseInt` + `sales >= 0`，补边界测试。
- [ ] #29 同步 README / architecture：删除 `failed` 状态、补 `avg_listed_price` / `avg_selling_price`、修正 grain 与“原始层不解析”的表述。
- [ ] #22 修正路由注释、`ListColumns` 首行样例、行数口径、`top-products` 的 `avg_price` 命名、client 超时配置。
- [ ] #14 `limit` 在 Go 侧统一收敛。
- [ ] #16 `start_month` / `end_month` 正则校验。
- [ ] #13 / #28 统一上游 4xx 与 handler 错误码映射。

### 第二批：直接影响报表正确性

- [ ] #24 手工写入不再接受客户端 `batch_id`，由服务端建 `manual` 批次。
- [ ] #5 残留：`ReplaceBatch` 与 `UpdateStatus` 同事务；删除 `StatusFailed`。
- [ ] #25 并发 `transform` 串行化（批次行锁 / advisory lock）。
- [ ] #26 属性 key 支持 Unicode 并增加 key 列表接口；默认白名单对齐实际数据。
- [ ] #9 对账覆盖全部度量 + 分页/局部对账。
- [ ] #30 原始层校验品类。

### 第三批：结构性调整

- [ ] #8 汇总层增量刷新 + 异步任务 + 复合索引。
- [ ] #7 手工写入纳入批次管理，补删除/回滚路径。
- [ ] #1 原始文件留存 + SHA-256 去重 + reader 版本。
- [ ] #17 补外键、`CHECK`、状态枚举。
- [ ] #18 删除交给外键或真正落地 Unit of Work；移除未使用的 `uow`。
- [ ] #11 双口径在路径/响应中显式区分，补 `source` 与 `as_of`。
- [ ] #27 上传接口线程池化、大小/行数限制、4xx 透传。

### 第四批：待需求确认后再定

- [ ] #20 Python + Pandas 是否保留。
- [ ] #19 是否引入 ADS 物化层。
- [ ] #10 grain 文档表述修正（若不做 `dws_sales_attrs`）。
- [ ] #21 补 handler / 事务 / 并发测试并接入 CI。

---

## 7. 需要先确认的问题

1. 分析侧是否存在 ML、复杂透视、非结构化处理需求？答案决定 #20 是“保留 Python”还是“合并回 Go”。
2. 报表是否需要复现历史口径？答案决定 #19 是否要引入 ADS 快照与 `as_of`。
3. `top-products` 的 `avg_price` 是否要改成“平均成交价”？#3 已在汇总接口拆开，但热销商品接口仍是挂牌价口径。
4. 手工写入（`POST /api/sales/batch`）是否仍需保留？若只是调试入口，可以直接下线，#7/#24 随之消失。
5. 是否会部署多实例？答案决定汇总层刷新与 `transform` 串行化是否需要分布式锁。
6. 原始文件留存的可接受成本是多少（数据库 bytea vs 对象存储）？答案决定 #1 的落盘方案。
7. `attrs` 的属性 key 是否需要支持中文列名？若需要，#26 不能只靠白名单解决。

---

*本文档随架构演进更新；最近复核：2026-10-31，基线 `4df6303`。*
