# 设计评审：分层数据平台（2026-11-02 复核更新）

> 基线：`2fcebfe`（2026-11-01，提交信息为“raw 层加入文件指纹识别，重复文件上传不会重复添加数据”）。
> 上一版评审基于 `4df6303`。本次复核确认原始文件留存与指纹去重已经落地，同时发现去重索引、上传完整性和原始文件生命周期等新问题。
> 本文保留原编号以便追溯，新增 31–34 号问题；每条结论都对应仓库中的实际代码或数据库结构。

## 0. 本轮复核说明

**复核范围**：`README.md`、`docs/architecture.md`、`docs/design-review.md`、`go_api/`、`python_code/`（不含 `.venv`）。

**复核方法**：

1. 逐项对照上一版评审与当前代码，确认“声称已改”是否真正落地。
2. 阅读 `2fcebfe` 的 diff，重点检查 `raw_files`、`/api/import-uploads` 与 Python 接入层。
3. 检查真实 PostgreSQL 的表结构与索引，不只依赖 GORM 模型声明。
4. 运行现有构建和测试。

**本轮执行结果**：

| 检查 | 结果 |
|---|---|
| `cd go_api && go build ./...` | 通过 |
| `cd go_api && go vet ./...` | 通过 |
| `cd go_api && go test ./...` | 通过；`mapping` 单测通过；`summary` 的 3 个集成测试因未设置 `TEST_DATABASE_URL` 跳过 |
| `cd python_code && uv run python -m unittest discover -s tests -v` | 8 个用例全部通过 |
| 数据库结构检查 | `raw_files` 已存在；`import_batches` 已增加文件字段；发现 `uk_batch_cat_file` 实际只约束 `raw_file_id` |
| 端到端上传验证 | 上一轮已实测：phone_sales.csv 上传、transform、dws_sales 对账通过；本轮未重复执行 |

**总体结论**：

- 原 #1 已基本修复：原始字节存入 `raw_files.content`，SHA-256 全局唯一，重复上传返回已有批次。
- 但原 #1 仍有缺口：没有重放接口、没有清理机制，且 Go 上传接口不校验 rows 是否真的来自文件。
- 新发现一个会直接破坏去重语义的问题：批次唯一索引建成了 `UNIQUE(raw_file_id)`，不是设计的 `UNIQUE(category, raw_file_id)`。
- 汇总层全量重建、对账不完整、手工数据不可删除、销量解析不安全等旧问题仍未处理。
- README 与 architecture 已在本次更新中同步到当前实现，文档漂移（原 #29）关闭。

等级定义：

- **P0**：数据错误、数据丢失或不可回溯，必须改。
- **P1**：数据量、并发或脏输入上来后会产生错误结果，需要改。
- **P2**：口径不一致、维护成本高、错误语义混乱，建议改。

状态标记：**已修复**、**部分修复**、**仍存在**、**本轮新增**。

---

## 1. 缺陷总览

| 编号 | 等级 | 问题 | 状态 | 位置 |
|---|---|---|---|---|
| 1 | P0 | 原始层不保存原始文件，解析结果不可重放 | 部分修复 | `raw_files`、`raw_records` |
| 2 | P0 | 金额用 `float64` / `float8` 存储 | 已修复 | `sale/model.go`、`summary/model.go` |
| 3 | P0 | `avg_price` 只有按记录数加权口径 | 已修复 | `python_code/app/modules/analytics/service.py` |
| 4 | P0 | `product_count` 用 `COUNT(DISTINCT name)` | 已修复 | `python_code/app/modules/analytics/service.py` |
| 5 | P0 | 转换失败后 `failed` 状态与上一轮数据矛盾 | 部分修复 | `go_api/internal/mapping/service.go` |
| 6 | P0 | `attrs` 只装未映射列 | 已修复 | `go_api/internal/mapping/service.go` |
| 7 | P0 | 手工写入（`batch_id` 为空）无删除路径 | 仍存在 | `sale/service.go`、`raw/repository.go` |
| 8 | P1 | 汇总层全量 `TRUNCATE` 重建同步跑在请求里 | 仍存在 | `summary/repository.go` |
| 9 | P1 | 对账只比 `total_sales`，无分页与上限 | 仍存在 | `summary/repository.go` |
| 10 | P1 | 文档称 grain 能支撑全部报表 | 已修正文档 | `docs/architecture.md` |
| 11 | P1 | 明细口径与汇总口径并存，响应不区分来源 | 仍存在 | `analytics/handler.go`、Python service |
| 12 | P1 | `transform` 一次性全量载入内存 | 仍存在 | `mapping/service.go` |
| 13 | P2 | 上游 4xx 被统一压成 502 | 仍存在 | `analytics/client.go` |
| 14 | P2 | `limit` 处理规则不一致 | 仍存在 | `analytics/handler.go` |
| 15 | P2 | `attributes` 的 `productor` 被静默丢弃 | 仍存在 | `python_code/app/api/analytics.py` |
| 16 | P2 | `start_month` / `end_month` 无格式校验 | 仍存在 | `analytics/handler.go` |
| 17 | P2 | 数据库层缺外键、`CHECK`、状态枚举 | 仍存在 | `sale/model.go`、`raw/model.go` |
| 18 | P2 | `PurgeBatch` 直接删 `sales`；`uow` 未使用 | 仍存在 | `raw/repository.go`、`common/uow` |
| 19 | P2 | 没有真正的 ADS 物化层 | 仍存在 | 整体架构 |
| 20 | P2 | Python + Pandas 主要承担纯 SQL 代理 | 仍存在 | 整体架构 |
| 21 | P2 | 测试偏纯函数，handler / 事务 / 并发无覆盖 | 部分修复 | `go_api/internal/*`、`python_code/tests` |
| 22 | P2 | 注释与实现不符（样例值、行数口径、路由注释） | 部分修复 | `raw/repository.go`、`router.go` |
| 23 | P1 | `transform` 用 `ParseFloat` 和 `int64` 解析销量，静默截断、接受负数或 NaN | 仍存在 | `mapping/transform.go` |
| 24 | P1 | `POST /api/sales/batch` 的 `batch_id` 未校验归属 | 仍存在 | `sale/service.go` |
| 25 | P1 | 同一批次并发 `transform` 可产生重复明细 | 仍存在 | `mapping/service.go`、`sale/repository.go` |
| 26 | P2 | `attrs` 的 key 是原始列名，中文 key 无法查询 | 仍存在 | `mapping/service.go`、Python service |
| 27 | P2 | Python 上传阻塞事件循环、先解析后去重、客户端错误语义不清 | 部分修复 | `python_code/app/api/importer.py` |
| 28 | P2 | 错误码映射不一致：转换失败一律 400，手工写入失败一律 500 | 仍存在 | `mapping/handler.go`、`sale/handler.go` |
| 29 | P2 | README / architecture 与实现漂移 | 已修复 | `README.md`、`docs/architecture.md` |
| 30 | P2 | `POST /api/imports` 不校验品类存在 | 仍存在 | `raw/service.go` |
| 31 | P1 | 批次唯一索引实际为 `UNIQUE(raw_file_id)`，同文件跨品类复用失败 | 本轮新增 | `raw/model.go`、数据库 `uk_batch_cat_file` |
| 32 | P1 | Go 上传接口不校验 rows 与文件内容一致，可提交任意行 | 本轮新增 | `raw/handler.go`、`raw/service.go` |
| 33 | P2 | 重复上传仍先解析；`raw_files` 无重放、无清理接口 | 本轮新增 | `importer.py`、`raw/repository.go` |
| 34 | P2 | Python `ImporterFaildError` 拼写错误且未被捕获，可能返回 500 | 本轮新增 | `python_code/app/modules/importer/service.py` |

---

## 2. 本轮确认已修复或明显改善

### 2.1 原始文件留存与指纹去重（原 #1）部分修复

现在上传链路会保存原始字节：

- `raw_files.content` 为 `bytea`，`raw_files.sha256` 有唯一索引。
- `import_batches` 增加 `file_hash`、`raw_file_id`、`reader_version`。
- Python 读取一次字节，用 `BytesIO(content)` 解析，再把同一份字节提交给 Go。
- Go 在 `CreateBatchFromFile` 中重新计算 SHA-256，以服务端结果为准。
- 同一文件同一品类重复上传返回已有批次，`duplicated=true`，不再新增 `raw_records`。

上一轮实测结果：phone_sales.csv 上传后 batch_id=13，168 行，transform 写入 `sales` 168 行，`dws_sales` 168 行，`summary/verify` 返回 `ok=true`；第二次上传返回同一 batch_id 和 `duplicated=true`。

**仍缺**：

- 没有从 `raw_files.content` 重放批次的接口。
- 删除批次不会删除 `raw_files`，也没有清理无引用文件的机制。
- Go 侧没有校验 `rows` 是否真的由该文件解析而来，见 #32。

**涉及文件**：`go_api/internal/raw/`、`python_code/app/api/importer.py`、`python_code/app/modules/importer/service.py`

### 2.2 金额精度（原 #2）已修复

`sales.price`、`dws_sales.total_amount / price_sum / min_price / max_price` 已使用 `decimal.Decimal` 与 PostgreSQL `numeric(18,4)`。

聚合 SQL 直接 `SUM(sales * price)`，由数据库按 numeric 计算。Python 只在出参时转 float 展示。

> 残留：`products.price` 仍是 `float64`，但只是品类参考价，不参与销售金额累计。

### 2.3 平均价口径（原 #3）已修复

`get_sales_summary` 同时返回：

```sql
SUM(price_sum) / SUM(record_count)  AS avg_listed_price
SUM(total_amount) / SUM(total_sales) AS avg_selling_price
```

旧字段 `avg_price` 保留为 `avg_listed_price` 的兼容别名。README 已同步说明。

> 残留：`top-products` 仍只返回 `avg_price`，语义是挂牌均价，不是成交均价。

### 2.4 `product_count`（原 #4）已修复

`get_sales_summary` 使用 `COUNT(DISTINCT (productor, name))`，跨品牌同名机型不再合并。

### 2.5 失败状态与数据矛盾（原 #5）部分修复

`mapping.Service.Transform` 不再在失败时写 `status=failed`。失败发生在写库之前，上一轮 `sales` 保持不变。

**残留**：

- `UpdateStatus` 在 `ReplaceBatch` 提交后单独执行，状态更新失败时明细已替换。
- `raw.StatusFailed` 是死常量；旧数据中仍可能有 `failed`。README 已注明 `failed` 仅历史数据。
- architecture 的状态图已移除 `loaded -> failed`。

### 2.6 `attrs` 内容（原 #6）已修复

`mapping/service.go` 将所有非标准列写入 `attrs`，与是否配置映射无关。补映射后重新 transform 不会丢属性。

### 2.7 文档同步（原 #10、#29）已修复

本次已更新：

- README：新增 `/api/import-uploads`，补充文件指纹、去重、`duplicated`、`file_hash`、`avg_listed_price` / `avg_selling_price`，修正 `failed` 状态与 transform 自动刷新说明。
- architecture：新增 `raw_files`，更新 ODS 定义、上传链路、事务边界、接口清单、数据库表、批次状态图，并修正 grain 与 ADS 表述。
- 原 #10 的“grain 支撑全部报表”已改为“支撑四类结构化报表，属性报表走明细探索口径”。

---

## 3. P0：仍存在的正确性与可回溯问题

### 3.1 手工写入的数据仍无删除路径（原 #7）仍存在

`POST /api/sales/batch` 仍允许 `batch_id` 为空直接写 `sales`。删除路径只覆盖：

```sql
DELETE FROM sales WHERE batch_id = ?
```

`batch_id IS NULL` 的行永远不会被命中。写错一次就会永久污染报表和汇总层。

**建议改法**：

- 为手工写入自动创建 `source=manual` 的批次，让所有明细都挂在批次下；或
- 提供按 `(productor, name, month)` 回滚明细的接口。

**涉及文件**：`go_api/internal/sale/service.go`、`sale/handler.go`、`raw/repository.go`

---

## 4. P1：仍存在的规模与正确性问题

### 4.1 汇总层全量重建跑在请求里（原 #8）仍存在

`summary.Repository.RefreshAll` 仍是单事务 `TRUNCATE + INSERT ... GROUP BY`，由 transform 和 delete 同步触发。

| 问题 | 说明 |
|---|---|
| O(N²) | 导入 N 个批次等于重建 N 次全表 |
| 读阻塞 | `TRUNCATE` 持 `ACCESS EXCLUSIVE` 锁到事务提交 |
| 请求超时 | 刷新耗时随累计数据量线性增长 |
| 缺索引 | `sales` 没有 `(product_id, productor, name, month)` 复合索引 |

**建议改法**：增量重算受影响 grain，用 `INSERT ... ON CONFLICT DO UPDATE`；刷新移出请求链路；补复合索引。

### 4.2 对账接口不完整（原 #9）仍存在

`summary.Repository.Verify` 只比较 `total_sales`：

- 未核对 `total_amount`、`price_sum`、`record_count`、`min_price`、`max_price`。
- 无 `limit`、无分页、无局部对账范围。
- 全表双向 `EXCEPT`，数据量大时可能超时或返回巨量 JSON。

**建议改法**：对账覆盖全部度量；增加 `limit` / `offset`；支持按批次或月份局部对账。

### 4.3 双口径并存且无闸门（原 #11）仍存在

`/api/analytics/attributes` 读 `sales`，其余四个接口读 `dws_sales`。响应体没有 `source` 和 `as_of`，路径也没有区分确认口径与探索口径。汇总滞后时，“总销量”和“按属性分组销量之和”可能对不上，系统不会提示。

**建议改法**：路径区分，如 `/api/analytics/explore/attributes`；响应增加 `source` 与 `as_of`。

### 4.4 transform 全量载入内存（原 #12）仍存在

`mapping/service.go` 仍一次读取全部 `raw_records`，构造全量 `[]sale.SaleRow` 后写入。峰值内存约为批次体积的 3 倍。

**建议改法**：按 `row_index` 游标分页流式处理，边解析边分批写入。

### 4.5 销量解析会静默截断、接受负数与 NaN（原 #23）仍存在

`mapping/transform.go`：

```go
salesNum, err := strconv.ParseFloat(values["sales"], 64)
...
Sales: int64(salesNum),
```

| 输入 | 结果 |
|---|---|
| `"12.9"` | `12`，静默截断 |
| `"-5"` | `-5`，负销量进入明细和汇总 |
| `"NaN"` / `"Inf"` | `ParseFloat` 成功，转 `int64` 结果依赖实现 |
| 超大整数 | 超过 `2^53` 后精度丢失 |

HTTP 的 `POST /api/sales/batch` 有 `gte=0`，但 transform 走自己的 `buildRow`，没有整数和非负校验。

**建议改法**：改用 `strconv.ParseInt`，显式校验 `>= 0`，补边界测试。

### 4.6 手工写入的 `batch_id` 未校验归属（原 #24）仍存在

`POST /api/sales/batch` 允许客户端自带 `batch_id`，服务端没有校验批次是否存在、品类是否匹配。后果：

1. 手工数据可以挂到其他批次下，删除批次时被一起删掉。
2. 重新 transform 该批次时，`ReplaceBatch` 的 `DELETE WHERE batch_id=?` 会删掉手工行。

**建议改法**：手工接口不接受客户端 `batch_id`，由服务端创建 `manual` 批次；或先校验批次和品类。

### 4.7 同一批次并发 transform 可产生重复明细（原 #25）仍存在

`ReplaceBatch` 是事务内先 `DELETE` 再 `INSERT`，`sales` 没有 `(batch_id, row_index)` 唯一约束。两个并发请求可能都完成 DELETE，然后各自 INSERT，导致明细翻倍。

**建议改法**：transform 事务内对 `import_batches` 行加 `SELECT ... FOR UPDATE`；或使用按 batch_id 的 advisory lock；或增加 `row_index` 唯一约束配合 upsert。

### 4.8 批次唯一索引建错列组合（本轮新增 #31）

模型声明：

```go
Category  string `json:"category" gorm:"not null;index"`
RawFileID *uint  `json:"raw_file_id" gorm:"uniqueIndex:uk_batch_cat_file,priority:2"`
```

`Category` 没有加入 `uk_batch_cat_file`，所以实际数据库索引是：

```text
uk_batch_cat_file UNIQUE btree (raw_file_id)
```

而不是设计的：

```text
UNIQUE (category, raw_file_id)
```

**影响**：

- 同一份文件用于不同品类时，需要复用 `raw_files` 并新建批次，但 `raw_file_id` 单列唯一会直接冲突。
- `CreateBatchWithFile` 在冲突后按 `(category, raw_file_id)` 查不到已有批次，返回记录不存在的错误。
- 去重语义从“品类内文件唯一”变成了“全局文件唯一”，与设计不符。

**建议改法**：

```sql
DROP INDEX uk_batch_cat_file;
CREATE UNIQUE INDEX uk_batch_cat_file
    ON import_batches(category, raw_file_id);
```

同时把模型改为：

```go
Category  string `json:"category" gorm:"not null;index;uniqueIndex:uk_batch_cat_file,priority:1"`
RawFileID *uint  `json:"raw_file_id" gorm:"uniqueIndex:uk_batch_cat_file,priority:2"`
```

补一条集成测试：同一文件分别以 `phone` 和 `computer` 上传，必须得到两个批次、一条 `raw_files`。

附带数据清理：旧批次的 `file_hash` 是 `char(64)` 填充的空格。建议执行：

```sql
UPDATE import_batches SET file_hash = '' WHERE btrim(file_hash) = '';
```

### 4.9 上传接口不校验 rows 与文件一致（本轮新增 #32）

`POST /api/import-uploads` 同时接收 `file` 和 `rows`。Go 只校验文件 SHA-256 和 rows 的结构，不校验 rows 是否由该文件解析而来。

**影响**：

- 调用方可以提交真实文件字节，但附带任意 rows；批次会被标记为该文件的解析结果。
- 由于去重按文件指纹生效，后续同一文件上传会直接返回这条被污染的批次，无法通过正常重传修正。
- 这不是身份认证问题，而是数据完整性边界问题。

**建议改法**（按强度从低到高）：

1. 将该接口限制为内部网络，并在网关层禁止外部直接访问。
2. Python 与 Go 共享内部签名密钥，对 `file_hash + rows_digest + category + reader_version` 做 HMAC。
3. Go 侧重新解析文件并与 rows 比对；这会引入 Go 解析器，改变当前分工。

短期建议至少实现第 1 或第 2 种，并在 README 中明确 `/api/import-uploads` 是内部接口。

---

## 5. P2：一致性与可维护性

### 5.1 上游 4xx 被压成 502（原 #13）

`analytics/client.go` 仍把上游所有 `>=300` 统一变成 `python returned %d`，handler 统一返回 `502 / 20001`。`top-products?limit=1000` 这类客户端错误会被报成分析服务不可用。

**建议改法**：4xx 映射回 400 并透传 `detail`，只在连接失败、超时、5xx 时返回 502。

### 5.2 limit 处理规则不一致（原 #14）

`top-products` 只兜底 `<=0`，没有上限；Python 侧上限是 500。`brands` 同理，Python 侧上限是 50。超出上限会触发 422，再被压成 502。

**建议改法**：Go 侧统一收敛到 Python 允许的范围，或直接返回 400。

### 5.3 attributes 的 productor 被静默丢弃（原 #15）

Go 的 `filterFrom` 会透传 `productor`，但 Python `/attributes` 没有该形参，FastAPI 忽略未知参数。调用方以为过滤生效，实际没有。

**建议改法**：Python 补 `productor` 并传给 `_filters`，或 Go 明确拒绝该参数。

### 5.4 时间参数无格式校验（原 #16）

`start_month` / `end_month` 全链路按字符串比较，没有正则校验。传 `2024-1` 会得到静默错误结果。

**建议改法**：Go handler 统一用 `^\d{4}-(0[1-9]|1[0-2])$` 校验，不合法返回 400。

### 5.5 数据库约束缺失（原 #17）

| 缺失项 | 后果 |
|---|---|
| `sales.product_id` 外键 | 可产生不存在品类的孤儿行 |
| `sales.batch_id` 外键 | 批次删除后可能残留明细 |
| `month` CHECK | 非标准格式可入库 |
| `status` 枚举 | 非法状态返回空列表而不是 400 |
| `import_batches.raw_file_id` 外键 | 可引用不存在的原始文件 |

**建议改法**：补外键和 CHECK；status 用 CHECK 或 PostgreSQL 枚举；handler 校验 status 白名单。

### 5.6 PurgeBatch 跨层删除，uow 未被使用（原 #18）

`raw.Repository.PurgeBatch` 直接执行 `DELETE FROM sales`，与“repository 独占数据访问”的架构声明冲突。`common/uow` 包没有任何引用。

**建议改法**：加 `ON DELETE CASCADE` 外键，让删除由数据库保证；或真正引入 Unit of Work；删除未使用的 `uow` 包。

### 5.7 没有真正的 ADS 层（原 #19）

`analytics` 是查询服务，不是物化的应用层结果表。报表口径没有版本，无法复现历史报表。

**建议**：短期文档已改为“查询服务”；长期为高频报表建立结果表或物化视图，并记录 `as_of`。

### 5.8 Python + Pandas 的收益需要确认（原 #20）

五个分析接口仍是纯 SQL 聚合，Pandas 的贡献主要是 `to_dict` 和 `iloc[0]`。成本包括多一个部署单元、多一次 HTTP 往返、schema 两份维护、Python 故障导致全部分析 502。

**建议**：确认是否存在 ML、复杂透视、非结构化处理；若没有，考虑把分析放回 Go；若有，给 Python 明确边界。

### 5.9 测试覆盖不足（原 #21）部分修复

| 模块 | 现有测试 |
|---|---|
| `go_api/internal/mapping` | 纯函数和假实现测试 |
| `go_api/internal/summary` | 3 个集成测试，缺 `TEST_DATABASE_URL` 时跳过 |
| `python_code/tests` | 8 个分析过滤用例 |
| 原始层 / 上传去重 | 无 |
| handler 层 | 无 |
| 事务 / 幂等 / 并发 | 无 |
| 销量解析边界 | 无 |

**建议改法**：补原始层去重集成测试、上传接口测试、`buildRow` 边界测试、并发 transform 测试，并接入 CI。

### 5.10 注释与口径不一致（原 #22）部分修复

| 位置 | 现状 |
|---|---|
| `router/router.go` | 注释仍写“获取 sales 表所有商品”，实际读 `products` |
| `raw/repository.go` `ListColumns` | 用 `MIN(row_data->>key)` 作样例值，取的是字典序最小值 |
| `GET /api/imports/:id` | 返回 `COUNT(raw_records)`，与批次 `row_count` 口径未统一说明 |
| `top-products` | 仍返回 `avg_price`，未跟随平均价拆分改名 |
| `config.go` | `python.timeout` 配置存在，但 client 硬编码 10s |

### 5.11 attrs 的 key 与查询白名单不匹配（原 #26）

`attrs` 的 key 是原始列名，但查询侧正则只允许 ASCII：

```python
_KEY_RE = re.compile(r"^[A-Za-z0-9_]{1,40}$")
```

中文列名会正常写入 `attrs`，但无法通过 `/api/analytics/attributes?key=屏幕` 查询，即使 `ALLOWED_ATTRS=*` 也不行。默认白名单也漏掉了 `computer_sales.csv` 里实际存在的 `memory`。

**建议改法**：允许 Unicode key；增加属性 key 列表接口；默认白名单与实际数据对齐。

### 5.12 上传链路的阻塞与错误语义（原 #27）部分修复

已改善：

- 增加 32MB 文件上限。
- 增加 SHA-256 与去重。
- 解析改用 `BytesIO(content)`，不再在文件游标末尾读取。
- Python 区分 `ImporterInvalidError` 和 `ImporterFailedError`。

仍存在：

- `/import/upload` 仍是 `async def`，内部直接调用阻塞的 pandas 和 requests，会卡住事件循环。
- 重复上传仍会先在 Python 解析，再去 Go 判重；重复文件白付一次解析成本。
- 解析异常虽然被捕获为 400，但其他请求异常仍可能变成 500。

**建议改法**：改为同步 `def` 或把解析放到线程池；增加按 `(category, file_hash)` 的预检查接口；统一异常映射。

### 5.13 错误码映射不一致（原 #28）

| 入口 | 行为 | 问题 |
|---|---|---|
| `mapping/handler.go` | 非 `ErrBatchNotFound` 一律 400 | 数据库错误、状态更新失败也被报成参数错误 |
| `sale/handler.go` | 所有错误一律 500 | 品类不存在、价格为负等客户端错误被报成服务器故障 |
| `raw/handler.go` | 创建批次失败一律 500 | 可预期错误与内部错误未区分 |

**建议改法**：定义 sentinel/typed error，handler 用 `errors.Is` 映射。

### 5.14 原始批次不校验品类（原 #30）

`POST /api/imports` 和 `/api/import-uploads` 都不检查 `category` 是否存在于 `products`。拼错品类会创建批次，直到 transform 才失败。

**建议改法**：创建批次前校验品类存在；或明确设计为原始层允许未知品类，并在响应中标注“待映射时校验”。

### 5.15 原始文件生命周期（本轮新增 #33）

- 重复上传仍先解析后去重。
- `raw_files` 只增不删；删除批次不会删除文件，也没有引用计数或保留期。
- 没有 `GET /api/raw-files/:sha256` 或 `POST /api/imports/:id/replay`，原始字节目前只能通过数据库访问。

**建议改法**：增加重放接口、受控下载接口和按保留期清理无引用文件的任务。

### 5.16 Python 异常类型拼写错误（本轮新增 #34）

`python_code/app/modules/importer/service.py` 定义了：

```python
class ImporterFaildError(Exception):
    ...
```

但 `import_file` 在 Go 返回 `code != 0` 时抛出的是 `ImporterFaildError`，API 层没有捕获它，最终会变成 500。正确名称应为 `ImporterFailedError`，并统一由 API 层捕获。

**建议改法**：删除拼写错误的类，统一使用 `ImporterFailedError`；补一个“Go 返回业务错误码”的单元测试。

---

## 6. 优先级建议

更细的实施步骤见 `docs/next-change-plan.md`。总体顺序：

1. **先修数据正确性**：批次索引（#31）、销量解析（#23）、手工 `batch_id`（#24）、并发 transform（#25）。
2. **再修汇总层**：增量刷新（#8）、对账覆盖（#9）、双口径标识（#11）。
3. **补生命周期**：手工数据删除（#7）、原始文件重放与清理（#1、#33）、数据库约束（#17）。
4. **最后统一 API 语义**：4xx 映射（#13）、limit（#14）、月份校验（#16）、错误码（#28）。
5. **持续补测试与 CI**（#21）。

---

## 7. 需要确认的问题

1. `/api/import-uploads` 是否只允许内部网络访问？答案决定 #32 是加网络限制还是加 HMAC 校验。
2. 同一文件用于不同品类是否必须支持？当前设计答案是“支持”，因此 #31 必须修。
3. 手工写入（`POST /api/sales/batch`）是否仍需保留？若只是调试入口，可以直接下线，#7、#24 随之消失。
4. `top-products` 的 `avg_price` 是否改成成交均价？还是新增 `avg_selling_price`？
5. 是否会部署多实例？决定汇总刷新和 transform 串行化是否需要分布式锁。
6. 原始文件保留多久？是否需要对象存储？决定 #33 的清理和迁移方案。

---

*本文档随代码演进更新；最近复核：2026-11-02，基线 `2fcebfe`。*
