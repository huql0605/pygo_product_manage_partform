# 设计评审：分层数据平台

> 本文是一次针对现有设计与实现的缺陷评审，目的是给出**明确的待修改清单**。

## 0. 评审说明

- 原始需求对话（DeepSeek 分享链接）需要登录，无法读取。本文结论全部基于仓库现有代码与 `docs/architecture.md` 的实际内容，每条都附带可核对的位置。
- 评审范围：`go_api/`、`python_code/`、`docs/architecture.md`、`README.md`。
- 结论：宏观分层思路（ODS → DWD → DWS、幂等转换、分子分母存法）成立，缺陷集中在**边界准确性**——文档声称的能力，代码有几处并未真正实现。

---

## 1. 缺陷总览

等级定义：

- **P0**：会导致数据错误、数据丢失或不可回溯，必须改。
- **P1**：数据量或并发上来后会失效，需要改。
- **P2**：口径不一致、维护成本高、错误语义混乱，建议改。

| 编号 | 等级 | 问题 | 位置 |
|---|---|---|---|
| 1 | P0 | 「原始层不解析」实际不成立，解析发生在 Python，且未保存原始文件 | `python_code/app/modules/importer/csv_reader.py` |
| 2 | P0 | 金额用 `float64` / `float8` 存储，累计产生精度误差 | `go_api/internal/sale/model.go`、`go_api/internal/summary/model.go` |
| 3 | P0 | `avg_price` 按记录数加权，口径可疑且缺「平均成交价」 | `python_code/app/modules/analytics/service.py` |
| 4 | P0 | `product_count` 用 `COUNT(DISTINCT name)`，跨品牌同名机型被合并 | `python_code/app/modules/analytics/service.py` |
| 5 | P0 | 转换失败后状态为 `failed`，但上一轮 `sales` 数据仍在，状态与数据矛盾 | `go_api/internal/mapping/service.go` |
| 6 | P0 | `attrs` 只装未映射列，改映射后重跑会导致属性数据消失 | `go_api/internal/mapping/service.go` |
| 7 | P0 | 手工写入（`batch_id` 为空）的销售数据无任何删除路径 | `go_api/internal/raw/repository.go`、`go_api/internal/sale/service.go` |
| 8 | P1 | 汇总层全量 `TRUNCATE` 重建同步跑在请求里，O(N²) 且阻塞分析读 | `go_api/internal/summary/repository.go` |
| 9 | P1 | 对账只比 `total_sales`，单价变化检测不到；无分页与上限 | `go_api/internal/summary/repository.go` |
| 10 | P1 | 文档称 grain 能支撑全部报表，但 `attributes` 报表需要 `attrs`，实际支撑不了 | `docs/architecture.md` 6.1 |
| 11 | P1 | 明细口径与汇总口径并存，无一致性校验、响应体不区分来源 | `python_code/app/modules/analytics/service.py` |
| 12 | P1 | `transform` 一次性全量载入内存，无流式分批 | `go_api/internal/mapping/service.go` |
| 13 | P2 | 上游 4xx 被统一压成 502，客户端错误报成服务端故障 | `go_api/internal/analytics/client.go` |
| 14 | P2 | `limit` 在三个分析接口上处理规则不一致，前两个必然触发第 13 条 | `go_api/internal/analytics/handler.go` |
| 15 | P2 | `attributes` 的 `productor` 参数被静默丢弃，与 README 不一致 | `python_code/app/api/analytics.py` |
| 16 | P2 | `start_month` / `end_month` 全链路无格式校验，写错静默返回错误结果 | `go_api/internal/analytics/handler.go`、`python_code/app/modules/analytics/service.py` |
| 17 | P2 | 数据库层无外键、无 `CHECK`、状态无枚举 | `go_api/internal/sale/model.go`、`go_api/internal/raw/model.go` |
| 18 | P2 | `PurgeBatch` 直接删 `sales` 表，破坏自身声明的分层约定 | `go_api/internal/raw/repository.go` |
| 19 | P2 | 没有真正的 ADS 层，报表每次现算，口径无版本 | `docs/architecture.md` 第 1 节 |
| 20 | P2 | Python + Pandas 承担的只是纯 SQL 代理，收益低、成本高 | 整体架构 |
| 21 | P2 | 测试只覆盖纯函数，幂等/事务/并发/HTTP 层无覆盖 | `go_api/internal/*/`、`python_code/tests/` |
| 22 | P2 | 注释与实现不符（`List`、`ListColumns` 样例值语义） | `go_api/internal/product/repository.go`、`go_api/internal/raw/repository.go` |

---

## 2. P0：正确性与可回溯

### 2.1 原始层其实已经被解析过

`docs/architecture.md` 的核心论断是：

> 用户输入永远先进原始层，永远不直接进标准层。

但真正的解析发生在 Python：

```python
pd.read_csv(source, dtype=str, encoding="utf-8-sig", keep_default_na=False)
```

这一步已经完成了编码归一（去 BOM）、类型强转、空值改写、分隔符推断。Go 侧所谓「原样存 JSONB」，存的是**Python 二次序列化后的结果**，既不是原始字节，也没有保留文件本身。

**影响**

- 原始文件丢弃后无法真正重放，解析规则变更、编码判断错误都无法修复历史批次。
- Excel 上传路径中，`openpyxl` 已把数值转成 float，精度损失发生在进入 ODS 之前。
- 没有文件指纹，导致同一文件重复上传无法识别（见第 7 条与第 1 条关联）。

**建议改法**

- 新增原始文件留存：`raw_files(sha256, filename, bytes | object_key, created_at)`。
- 批次表记录 `file_hash`，同一 `(category, file_hash)` 重复上传时返回已有批次而非新建。
- 解析产物降级为「可重算的派生结果」，并记录 reader 版本号。
- 文档改写：原始层的定义是「原始文件字节」，不是「Python 解析后的字典」。

**涉及文件**：`python_code/app/api/importer.py`、`python_code/app/modules/importer/csv_reader.py`、`go_api/internal/raw/`、`docs/architecture.md`

---

### 2.2 金额使用浮点数

`sale.Sales.Price`、`dws_sales.total_amount / price_sum / min_price / max_price` 全部是 Go `float64`，落库为 PostgreSQL `float8`。

**影响**

- `SUM(price * sales)` 在亿级金额上出现可见误差。
- 目前仅靠 Python 侧 `round(..., 2)` 掩盖，误差在库内依然存在，对账与导出会受到污染。

**建议改法**

- 存储类型改为 `numeric(18,4)`。
- Go 侧使用 decimal 类型（如 `shopspring/decimal`），只在出参序列化时转 float。
- 同步修改 `summary` 聚合 SQL 与 `analytics` 的取数转换。

**涉及文件**：`go_api/internal/sale/model.go`、`go_api/internal/sale/dto.go`、`go_api/internal/summary/model.go`、`go_api/internal/summary/repository.go`、`python_code/app/modules/analytics/service.py`

---

### 2.3 `avg_price` 口径需要拆分

当前定义：

```sql
COALESCE(SUM(price_sum) / NULLIF(SUM(record_count), 0), 0) AS avg_price
```

即**按记录数加权**的平均单价。一条销量 1 的记录与一条销量 100000 的记录权重相同，业务上几乎没有解释力。

**建议改法**

| 指标 | 公式 | 含义 |
|---|---|---|
| `avg_listed_price` | `SUM(price_sum) / SUM(record_count)` | 按记录数加权（保留现有语义，改名） |
| `avg_selling_price` | `SUM(total_amount) / SUM(total_sales)` | 按销量加权，业务常用 |

**涉及文件**：`python_code/app/modules/analytics/service.py`、`README.md`

---

### 2.4 `product_count` 计算错误

```sql
COUNT(DISTINCT name) AS product_count
```

`name` 未带品牌维度。不同品牌存在同名机型时（`Pro`、`X1`、`A1` 这类很常见）会被合并成一个「产品」。

**建议改法**

```sql
COUNT(DISTINCT (productor, name)) AS product_count
```

**涉及文件**：`python_code/app/modules/analytics/service.py`

---

### 2.5 失败状态与数据状态互相矛盾

`mapping.Service.Transform` 在失败时标记 `status=failed`，但上一轮成功转换写入的 `sales` 行**没有被清理**，`dws_sales` 中仍然包含这批数据。

**影响**

- `GET /api/imports` 显示 failed，`/api/analytics` 却把它计入统计。
- 排查问题时结论会被误导，这是最典型的坑。

**建议改法**

- 把 `ReplaceBatch` 与 `UpdateStatus` 放进同一事务，状态与数据同生共死。
- 或者引入 `transforming` 中间态，失败后回到上一个已生效版本，并在响应中明确「当前生效的是哪个版本」。

**涉及文件**：`go_api/internal/mapping/service.go`、`go_api/internal/sale/repository.go`、`go_api/internal/raw/repository.go`

---

### 2.6 `attrs` 是映射配置的副产物

`mapping/service.go` 中 `attrs` 只从 `ignored`（未匹配到标准字段的列）填充。

**影响**

- 今天未配映射，`屏幕` 列进入 `attrs`。
- 明天补一条映射再重跑 `transform`，`attrs["屏幕"]` 消失。
- 属性分析报表的结果随映射配置变化而漂移，且不可复现。

**建议改法**

- `attrs` 装入**所有非标准列**，与是否配置过映射无关。
- 映射规则只决定标准字段的取值来源，不决定 `attrs` 的内容。

**涉及文件**：`go_api/internal/mapping/service.go`、`go_api/internal/mapping/transform.go`

---

### 2.7 手工写入的数据永久无法删除

`sales.batch_id` 为 NULL 表示手工导入，而删除路径是 `DELETE FROM sales WHERE batch_id = ?`，不会命中 NULL。

结合 `POST /api/sales/batch`「追加写入、非幂等」的语义，等于**写错一次数据永久污染报表**。

**建议改法**

- 为手工写入也生成批次（`source=manual`），使所有明细都挂在批次下。
- 增加明细行的物理删除接口，或至少提供按 `(productor, name, month)` 的回滚能力。

**涉及文件**：`go_api/internal/sale/service.go`、`go_api/internal/raw/repository.go`、`go_api/internal/sale/handler.go`

---

## 3. P1：规模与性能

### 3.1 汇总层全量重建跑在请求里

`summary.Repository.RefreshAll` 是 `TRUNCATE + INSERT ... GROUP BY` 的单事务，被 `transform` 与 `delete` 两个请求**同步触发**。

**三重问题**

| 问题 | 说明 |
|---|---|
| O(N²) | 导入 N 个批次 = 重建 N 次全表，总代价随累计数据量平方增长 |
| 读阻塞 | `TRUNCATE` 持 `ACCESS EXCLUSIVE` 锁直到事务提交，期间所有分析查询被挡住 |
| 请求超时 | `transform` 耗时才随累计数据量线性增长，前端先超时而后端仍在跑 |

**建议改法**

- 改为增量重算受影响的 grain（`docs/architecture.md` 6.3 已给出正确思路）。
- 用 `INSERT ... ON CONFLICT (grain) DO UPDATE` 替代 `TRUNCATE`，避免排他锁。
- 把刷新移出请求链路，改为异步任务；`transform` 只返回「明细已写入，汇总刷新中」。
- 若短期无法改异步，至少把 `TRUNCATE` 换成 `DELETE` + 索引，降低锁级别。

**涉及文件**：`go_api/internal/summary/repository.go`、`go_api/internal/summary/service.go`、`go_api/internal/mapping/service.go`、`go_api/internal/raw/service.go`

---

### 3.2 对账接口不完整且不可用

`summary.Repository.Verify` 只比较 `total_sales`：

```sql
WITH detail AS (SELECT ..., SUM(sales) AS total_sales FROM sales GROUP BY ...)
```

**问题**

| 问题 | 说明 |
|---|---|
| 漏校验 | `total_amount`、`price_sum`、`record_count` 全部未核对，单价变化检测不到 |
| 无上限 | 全表双向 `EXCEPT`，无 `limit`、无分页，`differences` 可能是巨量 JSON |
| 无性能约束 | 数据量大时接口超时 |

**建议改法**

- 对账覆盖全部可加度量与分子分母。
- 增加 `limit` 与 `offset`，只返回前 N 条差异 + 总数。
- 大表场景改为抽样对账或按 `product_id` 分区对账。

**涉及文件**：`go_api/internal/summary/repository.go`、`go_api/internal/summary/service.go`、`go_api/internal/summary/handler.go`

---

### 3.3 grain 声明与报表需求不符

`docs/architecture.md` 6.1 中：

> 这是能支撑全部现有报表的最小维度组合。

但 `attributes` 报表依赖 `attrs`，而 `attrs` 不在 grain 中，因此它被迫改扫 `sales` 明细。该论断已不成立。

**建议改法**

- 修正文档表述：grain 支撑的是「结构化的四类报表」，属性类报表明确归为探索口径。
- 或者在 DWS 中增加 `dws_sales_attrs(key, value, ...)` 的辅助汇总表。

**涉及文件**：`docs/architecture.md` 第 6 节

---

### 3.4 双口径并存且无闸门

`/api/analytics/attributes` 读 `sales`，其余四个接口读 `dws_sales`。

**影响**

- 汇总滞后时，「总销量」与「按属性分组的销量之和」对不上，系统不提示。
- 口径分流仅靠文档约定，接口路径并列在同一前缀下，调用方容易混淆。

**建议改法**

- 路径显式区分，例如 `/api/analytics/explore/attributes`。
- 响应体增加 `source: "dws" | "detail"` 与 `as_of`（汇总层最后刷新时间）。

**涉及文件**：`go_api/internal/router/router.go`、`go_api/internal/analytics/handler.go`、`python_code/app/modules/analytics/service.py`

---

### 3.5 `transform` 全量载入内存

`ListRecords` 一次读全部 `raw_records`，转成 `[]map` 再转 `[]SaleRow`，最后 `CreateInBatches`。峰值内存约为批次体积的 3 倍。

`max=10000` 的行数上限只是掩盖了问题，不是解决。

**建议改法**

- 按 `row_index` 游标分页流式处理。
- 边解析边分批写入，避免同时持有三份全量数据。

**涉及文件**：`go_api/internal/mapping/service.go`、`go_api/internal/raw/repository.go`

---

## 4. P2：一致性与可维护性

### 4.1 上游 4xx 被压成 502

`analytics.Client.get` 中，非 200/404 一律返回 `python returned %d`，Go 侧统一返回 `502 / 20001`。

**实际后果**

| 场景 | 真实原因 | 用户看到 |
|---|---|---|
| `top-products?limit=1000` | 超出 Python 校验上限（422） | 分析服务不可用（502） |
| 上传超过 10000 行 | Go 拒绝（400） | 服务不可用（502），原因丢失 |

客户端错误被报成服务端故障，是排障成本最高的设计缺陷。

**建议改法**

- 把上游 4xx 映射回 Go 的 400 并透传 `detail`。
- 只在连接失败、超时、5xx 时返回 502。

**涉及文件**：`go_api/internal/analytics/client.go`

---

### 4.2 `limit` 处理规则不一致

| 接口 | Go 侧 | Python 侧 | 结果 |
|---|---|---|---|
| `top-products` | 只兜底 `<=0`，无上限 | `ge=1, le=500` | 超限返回 502 |
| `brands` | 只兜底 `<=0`，无上限 | `ge=1, le=50` | 超限返回 502 |
| `attributes` | 夹到 100 | `ge=1, le=100` | 正常 |

**建议改法**

- Go 侧统一做上下限收敛，收敛后再透传；或在 Go 侧直接返回 400 并给出允许范围。

**涉及文件**：`go_api/internal/analytics/handler.go`

---

### 4.3 `attributes` 的 `productor` 参数被静默丢弃

`python_code/app/api/analytics.py` 的 `/attributes` 没有 `productor` 形参，FastAPI 会忽略未知 query 参数；而 Go 侧 `filterFrom` 会照常透传。

`README.md` 写作「`productor` 适用全部接口」，与实际不符。

**建议改法**

- 要么在 Python 侧支持该参数（`_filters` 已有 `productor` 能力，只需透传）。
- 要么在 Go 侧明确拒绝未知参数并修正 README。

**涉及文件**：`python_code/app/api/analytics.py`、`python_code/app/modules/analytics/service.py`、`README.md`

---

### 4.4 时间参数全链路零校验

`start_month` / `end_month` 从 Go 到 Python 全程只用 `str`，没有正则校验。

传 `2024-1` 不会报错，而 `month >= '2024-1'` 在字符串比较下会产生**静默错误结果**。写入侧有正则校验，读取侧没有，两侧不对称。

**建议改法**

- 在 Go handler 层统一用 `^\d{4}-(0[1-9]|1[0-2])$` 校验，不合法返回 400。
- 或改用 `date` 类型的第一天存储，从类型上避免字符串比较问题。

**涉及文件**：`go_api/internal/analytics/handler.go`

---

### 4.5 数据库层零约束

| 缺失项 | 位置 | 后果 |
|---|---|---|
| `sales.product_id` 外键 | `go_api/internal/sale/model.go` | 可产生指向不存在品类的孤儿行 |
| `sales.batch_id` 外键 | `go_api/internal/sale/model.go` | 被删除批次的数据可能残留 |
| `month` 的 `CHECK` | `go_api/internal/sale/model.go` | 非标准格式可入库 |
| `status` 枚举 | `go_api/internal/raw/model.go` | `?status=随便` 返回空列表而非 400 |

`AutoMigrate` 因模型没有关联字段，不会自动建外键。

**建议改法**

- 补外键（`ON DELETE CASCADE` 或 `RESTRICT`，取决于业务语义）。
- 补 `CHECK (month ~ '^\d{4}-(0[1-9]|1[0-2])$')`。
- `status` 用 `CHECK IN (...)` 或 PG 枚举类型约束。

**涉及文件**：`go_api/internal/sale/model.go`、`go_api/internal/raw/model.go`、`go_api/internal/router/router.go`

---

### 4.6 `PurgeBatch` 破坏自身分层约定

`docs/architecture.md` 第 7 节声明：

> repository 独占数据访问
> 依赖方向（严格单向）

但 `raw.Repository.PurgeBatch` 直接执行 `DELETE FROM sales`。注释给出的理由（跨层删除需同一事务）是正确的，但这恰恰说明**层边界画错了**：这属于 Unit of Work 的职责。

另外 `mapping` 声称「消费方定义接口」，但 `SaleWriter` 的签名直接使用 `sale.SaleRow`、`RawReader` 使用 `raw.ImportBatch`，接口只隔离了方法，没有隔离类型。

**建议改法**

- 加外键 `ON DELETE CASCADE`，让删除由数据库保证，`PurgeBatch` 只删批次表。
- 或引入显式的事务协调者（Unit of Work），由上层组合多个 repository 调用。

**涉及文件**：`go_api/internal/raw/repository.go`、`go_api/internal/mapping/service.go`

---

### 4.7 没有真正的 ADS 层

文档把 `analytics` 报表等同于 ADS。但 ADS 的本义是**物化的**应用层结果表，而现在是每次请求现算。

**影响**

- 报表口径没有版本，无法复现历史报表。
- DWS 直接暴露给 API，后续若要加缓存、快照、导出，缺少落点。

**建议改法**

- 若短期不改，把文档表述改成「ADS = 查询服务」以免概念混淆。
- 若需长期演进，为高频报表建立结果表或物化视图。

**涉及文件**：`docs/architecture.md` 第 1 节

---

### 4.8 Python + Pandas 的收益需要重新确认

五个分析接口全部是纯 SQL 聚合，Pandas 的实际贡献只有 `.to_dict(orient="records")` 与 `.iloc[0]`。

**为此付出的成本**

| 成本 | 说明 |
|---|---|
| 部署单元 | 多一个服务要运维、要监控 |
| 网络跳转 | 每次分析多一次 HTTP 往返 |
| schema 重复 | Python 直接读 `sales` / `dws_sales` / `products`，与 GORM model 两份维护，无 contract test |
| 故障源 | Python 挂掉，全部分析接口返回 502 |
| 错误语义翻译 | 第 4.1 条因此存在 |
| 连接池竞争 | Python 与 Go 抢同一个 PostgreSQL 连接，而刷新持锁时正是分析最需要连接的时候 |

**建议**

- 回到原始需求确认：是否存在 ML、复杂透视、非结构化处理的诉求。
- 若没有，考虑把分析直接放在 Go 侧。
- 若有，应给 Python 明确定位（只做需要 Python 生态的分析），而不是继续代理纯 SQL。

---

### 4.9 测试覆盖不足

| 模块 | 现有测试 |
|---|---|
| `go_api/internal/mapping` | `service_test.go`、`transform_test.go`（纯函数） |
| `go_api/internal/summary` | `service_test.go` |
| `python_code/tests` | `test_analytics_filters.py` |
| handler 层 | 无 |
| 事务 / 幂等 / 并发 | 无 |

集成测试依赖 `TEST_DATABASE_URL`，缺失时直接跳过。

**建议改法**

- 补 handler 层测试（参数校验、错误码映射）。
- 补幂等测试（同一批次重复 `transform` 结果一致）。
- 补并发测试（并发刷新与查询不产生错误数据）。

**涉及文件**：`go_api/internal/*/`、`python_code/tests/`

---

### 4.10 注释与实现不符

| 位置 | 现状 |
|---|---|
| `go_api/internal/product/repository.go` | 注释写「获取 sales 表所有商品」，实际读 `products` 表 |
| `go_api/internal/raw/repository.go` `ListColumns` | 用 `MIN(row_data->>key)` 作「样例值」，取到的是字典序最小值，不是真实首例 |
| `GET /api/imports/:id` | 返回 `COUNT(raw_records)`，而批次表已有 `row_count`，两者可能不一致 |

**建议改法**

- 修正注释。
- `ListColumns` 改为取真实首行值（如 `ORDER BY row_index` + `DISTINCT ON`）。
- 行数统一口径，二选一并说明。

---

## 5. 修改路线

按投入产出排序，前两项风险最低、收益最大。

### 第一批：纯增量，不影响现有数据

- [ ] 4.1 错误语义分层（上游 4xx 映射回 400 并透传 detail）
- [ ] 4.2 `limit` 统一收敛
- [ ] 4.4 时间参数正则校验
- [ ] 4.3 `attributes` 支持 `productor`，或修正 README
- [ ] 2.4 `product_count` 改 `COUNT(DISTINCT (productor, name))`
- [ ] 4.10 修正注释与样例值语义

### 第二批：直接影响报表正确性

- [ ] 2.3 `avg_price` 拆成两个口径
- [ ] 2.5 状态与数据放进同一事务
- [ ] 2.6 `attrs` 改为装全部非标准列
- [ ] 3.2 对账覆盖全部度量 + 分页
- [ ] 2.2 金额改 `numeric`

### 第三批：结构性调整

- [ ] 3.1 汇总层改增量 + 异步
- [ ] 2.7 手工写入纳入批次管理，补删除路径
- [ ] 2.1 原始文件留存 + 文件指纹去重
- [ ] 4.5 补外键与 CHECK 约束
- [ ] 4.6 删除路径交给外键或 Unit of Work
- [ ] 3.4 双口径在接口层面显式区分

### 第四批：待需求确认后再定

- [ ] 4.8 Python + Pandas 是否保留
- [ ] 4.7 是否引入 ADS 物化层
- [ ] 3.3 文档中 grain 的表述修正
- [ ] 4.9 补齐测试

---

## 6. 需要先确认的问题

以下问题会影响修改方案的选择，建议先与需求方确认：

1. 分析侧是否存在 ML、复杂透视、非结构化处理需求。答案决定第 4.8 条是「保留」还是「合并」。
2. 报表是否需要复现历史口径。答案决定是否要引入 ADS 快照。
3. 「平均单价」的业务定义是哪一个。答案决定第 2.3 条是「新增」还是「替换」。
4. 手工写入（`POST /api/sales/batch`）是否仍需保留。若只是调试入口，可以直接下线，第 2.7 条随之消失。
5. 是否会部署多实例。答案决定汇总层刷新是否需要分布式锁。
