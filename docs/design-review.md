# 设计评审：分层数据平台（2026-11-03 复核更新）

> 基线：`6e4fa82`（2026-11-03，加入 transform 互斥锁，处理并发情况下 transform 的异常）。
> 本次复核确认 #23、#25、#31、#34 以及 #5 的状态事务问题已经修复；原始文件留存（#1）仍有缺口，上传完整性（#32）已于 2026-11-06 修复。
> 本文保留原编号以便追溯，新增 31–35 号问题；每条结论都对应仓库中的实际代码或数据库结构。

## 0. 本轮复核说明

**复核范围**：`README.md`、`docs/architecture.md`、`docs/design-review.md`、`go_api/`、`python_code/`（不含 `.venv`）。

**复核方法**：

1. 逐项对照上一版评审与当前代码，确认“声称已改”是否真正落地。
2. 阅读 `6e4fa82` 的 diff，重点检查 transform 事务、批次锁与并发行为。
3. 检查真实 PostgreSQL 的表结构与索引，不只依赖 GORM 模型声明。
4. 运行现有构建和测试，并做并发 transform 与锁等待验证。

**本轮执行结果**：

| 检查 | 结果 |
|---|---|
| `cd go_api && go build ./...` | 通过 |
| `cd go_api && go vet ./...` | 通过 |
| `cd go_api && go test ./...` | 通过；`mapping` 单测通过；`summary` 的 3 个集成测试因未设置 `TEST_DATABASE_URL` 跳过 |
| `cd python_code && uv run python -m unittest discover -s tests -v` | 8 个用例全部通过 |
| 数据库结构检查 | `uk_batch_cat_file` 已修正为 `UNIQUE (category, raw_file_id)` |
| 并发 transform 验证 | 4 个并发请求后 `sales` 行数保持 168，状态为 `mapped`，汇总对账通过 |
| 锁等待验证 | 手动持有批次行锁后，transform 5 秒超时返回 `409 / 10005` |

**总体结论**：

- 原 #1 已基本修复：原始字节存入 `raw_files.content`，SHA-256 全局唯一，重复上传返回已有批次。
- 原 #1 仍有缺口：没有重放接口、没有清理机制，且 Go 上传接口不校验 rows 是否真的来自文件。
- #23 销量整数解析、#25 并发 transform、#31 批次唯一索引、#34 Python 异常类拼写、#5 状态事务均已修复。
- 汇总层全量重建、对账不完整、手工数据不可删除、上传 rows 完整性等旧问题仍未处理。
- README 与 architecture 已在上一轮同步；本轮又修正了批次索引与旧 `file_hash` 的说明。

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
| 5 | P0 | 转换失败后 `failed` 状态与上一轮数据矛盾 | 已修复 | `go_api/internal/mapping/service.go` |
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
| 18 | P2 | `PurgeBatch` 直接删 `sales`，跨层职责未收敛 | 仍存在 | `raw/repository.go` |
| 19 | P2 | 没有真正的 ADS 物化层 | 仍存在 | 整体架构 |
| 20 | P2 | Python + Pandas 主要承担纯 SQL 代理 | 仍存在 | 整体架构 |
| 21 | P2 | 测试偏纯函数，handler / 事务 / 并发无覆盖 | 部分修复 | `go_api/internal/*`、`python_code/tests` |
| 22 | P2 | 注释与实现不符（样例值、行数口径、路由注释） | 部分修复 | `raw/repository.go`、`router.go` |
| 23 | P1 | `transform` 用 `ParseFloat` 和 `int64` 解析销量，静默截断、接受负数或 NaN | 已修复 | `mapping/transform.go` |
| 24 | P1 | `POST /api/sales/batch` 的 `batch_id` 未校验归属 | 仍存在 | `sale/service.go` |
| 25 | P1 | 同一批次并发 `transform` 可产生重复明细 | 已修复 | `mapping/service.go`、`raw/repository.go` |
| 26 | P2 | `attrs` 的 key 是原始列名，中文 key 无法查询 | 仍存在 | `mapping/service.go`、Python service |
| 27 | P2 | Python 上传阻塞事件循环、先解析后去重、客户端错误语义不清 | 部分修复 | `python_code/app/api/importer.py` |
| 28 | P2 | 错误码映射不一致：转换失败一律 400，手工写入失败一律 500 | 仍存在 | `mapping/handler.go`、`sale/handler.go` |
| 29 | P2 | README / architecture 与实现漂移 | 已修复 | `README.md`、`docs/architecture.md` |
| 30 | P2 | `POST /api/imports` 不校验品类存在 | 仍存在 | `raw/service.go` |
| 31 | P1 | 批次唯一索引实际为 `UNIQUE(raw_file_id)`，同文件跨品类复用失败 | 已修复 | `raw/model.go`、数据库 `uk_batch_cat_file` |
| 32 | P1 | Go 上传接口不校验 rows 与文件内容一致，可提交任意行 | 已修复 | `raw/handler.go`、`raw/service.go` |
| 33 | P2 | 重复上传仍先解析；`raw_files` 无重放、无清理接口 | 本轮新增 | `importer.py`、`raw/repository.go` |
| 34 | P2 | Python `ImporterFaildError` 拼写错误且未被捕获，可能返回 500 | 已修复 | `python_code/app/modules/importer/service.py` |
| 35 | P2 | `PurgeBatch` 加锁但没有锁等待上限 | 已修复 | `raw/repository.go`、`raw/service.go`、`raw/handler.go` |

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

### 2.5 失败状态与数据矛盾（原 #5）已修复

`mapping.Service.Transform` 已把批次加锁、原始行读取、明细替换和状态更新放进同一个事务：

- 任意一步失败都会整体回滚，不会出现“明细已替换、状态还是旧值”。
- 转换失败发生在写库之前，上一轮 `sales` 保持不变。
- 不再写 `status=failed`；`raw.StatusFailed` 只用于兼容旧数据。

README 已注明 `failed` 仅可能出现在历史数据中，architecture 的状态图已移除 `loaded -> failed`。

### 2.6 `attrs` 内容（原 #6）已修复

`mapping/service.go` 将所有非标准列写入 `attrs`，与是否配置映射无关。补映射后重新 transform 不会丢属性。

### 2.7 文档同步（原 #10、#29）已修复

本次已更新：

- README：新增 `/api/import-uploads`，补充文件指纹、去重、`duplicated`、`file_hash`、`avg_listed_price` / `avg_selling_price`，修正 `failed` 状态与 transform 自动刷新说明。
- architecture：新增 `raw_files`，更新 ODS 定义、上传链路、事务边界、接口清单、数据库表、批次状态图，并修正 grain 与 ADS 表述。
- 原 #10 的“grain 支撑全部报表”已改为“支撑四类结构化报表，属性报表走明细探索口径”。

### 2.8 上传 rows 完整性（原 #32）已修复

`/api/import-uploads` 现在要求内部 HMAC 签名：

- Python 对 `file_hash + "\n" + category + "\n" + reader_version + "\n" + sha256(rows_json)` 计算 HMAC-SHA256，`rows_json` 使用固定 separators 和 `sort_keys`。
- Go 对收到的 rows 表单原文计算摘要，签名校验通过后才写 `raw_files` / `import_batches` / `raw_records`。
- 两边共用 `IMPORT_SIGNING_KEY`，任一进程缺少该变量时启动失败。

2026-11-06 实测：

- Python 客户端正常上传返回 `code=0`，写入 1 个批次。
- 篡改 rows、缺失 `signature`、错误 key 均返回 `400 / 签名不一致`，数据库中无对应批次。

### 2.9 PurgeBatch 锁等待上限（原 #35）已修复

`PurgeBatch` 在加锁前执行 `SET LOCAL lock_timeout = '5s'`。`55P03` 通过 `errors.As(err, &pgErr)` 映射为 `ErrBatchBusy`，handler 返回 `409 / 10005`。

本次同时修了两个会导致错误处理失效的问题：

- 原代码写成 `errors.As(pgErr, &pgErr)`，把 nil 指针作为 `error` 传入，锁超时后触发 nil pointer panic，最终返回 500。
- handler 缺少一般错误分支，非 `ErrBatchBusy` / `ErrBatchNotFound` 的错误会被吞掉。现已补 `response.InternalError`。

2026-11-06 实测：外部持有 `import_batches.id=13` 行锁后，`DELETE /api/imports/13` 在 5.004 秒返回 `409 / 10005`，批次未被删除；删除不存在批次返回 `404 / 10002`。

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

### 4.5 销量解析会静默截断、接受负数与 NaN（原 #23）已修复

`mapping/transform.go` 已改为：

```go
salesNum, err := strconv.ParseInt(values["sales"], 10, 64)
if err != nil {
    return nil, fmt.Errorf("第 %d 行 sales 不是数字: %q", lineNo, values["sales"])
}
if salesNum < 0 {
    return nil, fmt.Errorf("第 %d 行 sales 不能为负: %d", lineNo, salesNum)
}
```

小数、负数、`NaN`、`Inf` 和超出 `int64` 的值都会被拒绝；测试已同步为“小数报错”。

### 4.6 手工写入的 `batch_id` 未校验归属（原 #24）仍存在

`POST /api/sales/batch` 允许客户端自带 `batch_id`，服务端没有校验批次是否存在、品类是否匹配。后果：

1. 手工数据可以挂到其他批次下，删除批次时被一起删掉。
2. 重新 transform 该批次时，`ReplaceBatch` 的 `DELETE WHERE batch_id=?` 会删掉手工行。

**建议改法**：手工接口不接受客户端 `batch_id`，由服务端创建 `manual` 批次；或先校验批次和品类。

### 4.7 同一批次并发 transform 可产生重复明细（原 #25）已修复

`mapping.Service.Transform` 现在在事务内先执行：

```sql
SET LOCAL lock_timeout = '5s';
SELECT * FROM import_batches WHERE id = ? FOR UPDATE;
```

然后读取原始行、替换明细、更新状态并提交。同一批次的两个 transform 会串行执行，第二个请求在第一个提交后才能继续。

实测 4 个并发 transform 后，`sales` 行数保持 168，没有翻倍；锁等待超过 5 秒返回 `409 / 10005`。

### 4.8 批次唯一索引建错列组合（原 #31）已修复

模型已改为：

```go
Category  string `json:"category" gorm:"not null;index;uniqueIndex:uk_batch_cat_file,priority:1"`
RawFileID *uint  `json:"raw_file_id" gorm:"uniqueIndex:uk_batch_cat_file,priority:2"`
```

数据库索引已从 `UNIQUE (raw_file_id)` 修正为：

```text
uk_batch_cat_file UNIQUE btree (category, raw_file_id)
```

旧批次的 `file_hash` 目前为空串，不再有 64 个空格的展示问题。

仍需补一条集成测试：同一文件分别以 `phone` 和 `computer` 上传，必须得到两个批次、一条 `raw_files`。

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

### 5.6 PurgeBatch 跨层删除（原 #18）仍存在

`raw.Repository.PurgeBatch` 直接执行 `DELETE FROM sales`，与“repository 独占数据访问”的架构声明冲突。

`common/uow` 现在已被 `mapping` 使用，不再是从未引用的死代码；但 `PurgeBatch` 仍没有走 Unit of Work。

**建议改法**：加 `ON DELETE CASCADE` 外键，让删除由数据库保证；或把跨 repository 事务提升到 service 层。

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

### 5.16 Python 异常类型拼写错误（原 #34）已修复

`python_code/app/modules/importer/service.py` 已删除 `ImporterFaildError`，统一使用 `ImporterFailedError`。API 层继续捕获该异常并返回 502，不会再因为未捕获的异常变成 500。

建议后续补一个“Go 返回业务错误码”的单元测试。

---

## 6. 优先级建议

更细的实施步骤见 `docs/next-change-plan.md`。总体顺序：

1. **先修数据正确性**：手工 `batch_id`（#24）、手工数据删除（#7）。
2. **再修汇总层**：增量刷新（#8）、对账覆盖（#9）、双口径标识（#11）。
3. **补生命周期**：原始文件重放与清理（#1、#33）、数据库约束（#17）。
4. **最后统一 API 语义**：4xx 映射（#13）、limit（#14）、月份校验（#16）、错误码（#28）。
5. **持续补测试与 CI**（#21）。

---

## 7. 需要确认的问题

1. `/api/import-uploads` 已采用内部 HMAC；是否还需要在网关层限制为只允许内网访问？
2. 同一文件用于不同品类是否必须支持？当前设计答案是“支持”，#31 已在索引层修复，仍需补集成测试。
3. 手工写入（`POST /api/sales/batch`）是否仍需保留？若只是调试入口，可以直接下线，#7、#24 随之消失。
4. `top-products` 的 `avg_price` 是否改成成交均价？还是新增 `avg_selling_price`？
5. 是否会部署多实例？决定汇总刷新和 transform 串行化是否需要分布式锁。
6. 原始文件保留多久？是否需要对象存储？决定 #33 的清理和迁移方案。

---

*本文档随代码演进更新；最近复核：2026-11-06，基线 `6e4fa82` 加 #32 验收。*
