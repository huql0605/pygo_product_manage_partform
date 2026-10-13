# 架构说明：分层数据平台

> 一句话定位：**Go + Python 构建的数据接入与分析平台，采用 ODS → DWD → ADS 分层思想，实现插件化接入与配置化字段映射。**

技术栈：Go（Gin + GORM + PostgreSQL）负责 API 与事务；Python（FastAPI + Pandas）负责分析与接入；两者通过 REST 解耦，`X-Request-ID` 跨语言串联日志。

---

## 1. 分层总览

```mermaid
flowchart LR
    CSVS["CSV / Excel / API"] --> ACC["接入层<br/>importer"]
    ACC --> RAW["原始层<br/>raw_records (JSONB)"]
    RAW --> MAP["映射层<br/>field_mappings"]
    MAP --> STD["标准层<br/>sales"]
    STD --> ANA["分析层<br/>analytics"]

    classDef layer fill:#eef,stroke:#668
    class ACC,RAW,MAP,STD,ANA layer
```

| 层 | 职责 | 变化频率 |
|---|---|---|
| 接入层 | 把文件读成「行」 | 加新数据源时改 |
| 原始层 | **原样存 JSONB，不解析** | 几乎不变 |
| 映射层 | 用户列名 → 系统标准字段 | 每个来源可不同 |
| 标准层 | 统一结构，供分析 | 加新字段时改 |
| 分析层 | 聚合、报表 | 加新指标时改 |

**核心洞察**：用户输入永远先进原始层，**永远不直接进标准层**。这是解决「表格一变就出 bug」的根本方法。

**与数仓分层的对应**：

| 数仓分层 | 本项目 |
|---|---|
| ODS（原始层） | `raw_records` |
| DWD（明细层） | 映射后的明细 |
| DWS / ADS（汇总/应用） | `sales` 聚合 + `analytics` 报表 |

---

## 2. 端到端流程（六个阶段）

```mermaid
flowchart TD
    CSV["phone_sales.csv<br/>productor,name,price,sales,month"]

    subgraph L1["① 接入层 · Python"]
        P1["pd.read_csv(dtype=str, keep_default_na=False)<br/>不改列名、不转类型"]
        P2["df.to_dict(orient=records)<br/>每行变成字典"]
        P3["POST /api/imports<br/>source, category, rows"]
        P1 --> P2 --> P3
    end

    subgraph L2["② 原始层 · Go — 只搬运，不解析列名"]
        R1["raw.Handler.CreateImport<br/>ShouldBindJSON + Validate"]
        R2["raw.Service.CreateBatch<br/>逐行 json.Marshal + 编号 row_index"]
        R3["事务 repo.CreateBatch"]
        B1[("import_batches<br/>id, source, category<br/>status=loaded, row_count")]
        B2[("raw_records<br/>batch_id, row_index<br/>row_data jsonb")]
        R1 --> R2 --> R3
        R3 --> B1
        R3 --> B2
    end

    subgraph L3["③ 看清有哪些列"]
        C1["GET /api/imports/:id/columns"]
        C2["jsonb_object_keys(row_data)<br/>CROSS JOIN LATERAL + GROUP BY"]
        C1 --> C2
    end

    subgraph L4["④ 映射层 · 配置规则"]
        M1["POST /api/mappings<br/>category, source_column, target_field"]
        M2["白名单校验<br/>productor/name/price/sales/month"]
        M3[("field_mappings<br/>唯一 category + source_column")]
        M1 --> M2 --> M3
    end

    subgraph L5["⑤ 映射层 · 转换 — 真正解析列名的地方"]
        T1["POST /api/imports/:id/transform"]
        T2["读 category + 映射规则"]
        T3["读 raw_records 全部行"]
        T4["逐行 resolveRow<br/>1 显式规则<br/>2 恒等回退<br/>3 未匹配记为 ignored"]
        T5["buildRow<br/>price 转 float, sales 转 int64<br/>month 校验 YYYY-MM"]
        T6["事务 sale.Service.ReplaceBatch<br/>DELETE WHERE batch_id=?<br/>再 INSERT"]
        T7["import_batches.status = mapped"]
        T1 --> T2 --> T3 --> T4 --> T5 --> T6 --> T7
    end

    SALES[("sales<br/>product_id, batch_id<br/>productor, name, price<br/>sales, month")]

    subgraph L6["⑥ 分析层"]
        A1["GET /api/analytics/*"]
        A2["Go analytics.Client<br/>透传 X-Request-ID"]
        A3["Python FastAPI<br/>pandas 读 sales"]
        A1 --> A2 --> A3
    end

    CSV --> P1
    P3 --> R1
    B2 --> C1
    B2 --> T3
    M3 --> T2
    T6 --> SALES
    SALES --> A1
```

---

## 3. 数据形态流转

同一条数据一路的形态变化：

```mermaid
flowchart LR
    S1["CSV 行<br/>Apple,iPhone 15 Pro,<br/>7114.41,11556,2024-01"]
    S2["pandas dict<br/>值是字符串"]
    S3["jsonb 存储<br/>键被重排<br/>值仍是字符串"]
    S4["resolveRow<br/>列名转标准字段名"]
    S5["SaleRow<br/>类型已转换"]
    S6["sales 表行<br/>+ product_id, batch_id"]

    S1 --> S2 --> S3 --> S4 --> S5 --> S6
```

| 阶段 | 数据形态 | **列名状态** | 值类型 |
|---|---|---|---|
| ① CSV | 文本行 | `productor,name,price,sales,month` | 文本 |
| ① pandas | `DataFrame` | 原样 | 文本（`dtype=str`） |
| ① dict | `{productor: Apple, ...}` | 原样 | 文本 |
| ② HTTP JSON | 同 dict | 原样 | 文本 |
| ② `raw_records.row_data` | **jsonb** | **原样**（jsonb 重排键顺序） | 文本 |
| ⑤ `resolveRow` | `map[string]string` | **翻译成标准字段名** | 文本 |
| ⑤ `buildRow` | `sale.SaleRow` | 标准字段 | **已转换** |
| ⑤ `sales` 行 | 关系表列 | 标准字段 | 关系类型 |

---

## 4. 关键设计：列名被「处理」两次

```mermaid
flowchart TD
    K1["CSV 的列名<br/>品牌 / 厂商 / productor"]
    K2["原始层<br/>不解析 — 原样存进 row_data"]
    K3["映射层<br/>翻译 — 品牌 转 productor"]
    K1 --> K2 --> K3
```

| | 原始层 | 映射层 |
|---|---|---|
| 对列名的态度 | **完全不碰** | **全部翻译** |
| 依据 | 无 | `field_mappings` + 恒等回退 |
| 列名出现在哪 | `row_data` 的**键** | 转换逻辑的**输入** |
| 目的 | 保留真相，任意表格都能存 | 产出统一结构，分析层不关心来源 |

### 列名解析的三级优先级

```
1. 显式规则      field_mappings: source_column → target_field
2. 恒等回退      源列名本身就在标准字段白名单里
3. 未匹配        记入 ignored_columns（数据不丢，只是本次转换用不上）
```

**「多来源 → 同一目标字段」是被允许的**（不同商家用不同写法），所以：

- 唯一约束是 `(category, source_column)`，**故意不加** `target_field` 唯一约束
- 转换时按「这一行里实际出现了哪个源列」解析
- 若一行里出现两个源列指向同一目标字段 → **报错**，不静默随机取值

### 列顺序无关

全程按**列名**匹配，与顺序无关：

- Python：`to_dict(orient="records")` → 字典，顺序已丢
- 存储：`jsonb` 会重排键（按长度 + 字典序）
- Go：`map[string]any` + `range` 迭代顺序本身随机

---

## 5. 事务边界

```mermaid
flowchart LR
    subgraph TX1["事务 1 · 写入原始层"]
        A1["INSERT import_batches"] --> A2["INSERT raw_records ×N"]
    end
    subgraph TX2["事务 2 · 替换标准层"]
        B1["DELETE sales WHERE batch_id=?"] --> B2["INSERT sales ×N"]
    end
```

| 事务 | 语义 | 收益 |
|---|---|---|
| 原始层写入 | 批次头 + 全部原始行，**全成或全败** | 不会出现「有批次没数据」 |
| 标准层替换 | **先删后插** | **幂等**：同批次重复转换不翻倍 |

转换失败时：整批不写，`sales` 保持原样，批次状态不变。任何一行的问题都会带**行号**返回。

---

## 6. 代码位置对照

| 阶段 | 入口 | 核心逻辑 |
|---|---|---|
| ① 接入 | `python_code/import_raw.py` | `pd.read_csv(dtype=str)` + `to_dict` |
| ② 原始层 | `go_api/internal/raw/handler.go` → `service.go` | `CreateBatch` + `repo.CreateBatch` |
| ③ 列名 | `go_api/internal/raw/repository.go` | `ListColumns`（`jsonb_object_keys`） |
| ④ 规则 | `go_api/internal/mapping/handler.go` → `service.go` | `Upsert` + 白名单校验 |
| ⑤ 转换 | `go_api/internal/mapping/` | `Transform` → `resolveRow` / `buildRow` |
| ⑥ 分析 | `go_api/internal/analytics/` → `python_code/app/` | HTTP 代理 + pandas 聚合 |

### 依赖方向（严格单向）

```mermaid
flowchart LR
    ROUTER["router"] --> HANDLER["handler"]
    HANDLER --> SERVICE["service"]
    SERVICE --> REPO["repository"]
    REPO --> MODEL["model"]
```

- handler 只做 HTTP 入参/出参
- service 承载业务规则与事务边界
- repository 独占数据访问
- 跨模块依赖由**消费方定义接口**（如 `mapping.RawReader` / `mapping.SaleWriter`），实现方隐式满足

---

## 7. 接口清单

### 原始层

| 方法 | 路径 | 作用 |
|---|---|---|
| `POST` | `/api/imports` | 写入原始数据（原样，不解析） |
| `GET` | `/api/imports/:id/columns` | 列出该批次出现过的列名 + 样本值 |

### 映射层

| 方法 | 路径 | 作用 |
|---|---|---|
| `POST` | `/api/mappings` | 配置/覆盖一条映射规则（upsert） |
| `GET` | `/api/mappings?category=` | 查看某品类的规则 |
| `POST` | `/api/imports/:id/transform` | 原始层 → 标准层转换（幂等） |

### 标准层 / 分析层

| 方法 | 路径 | 作用 |
|---|---|---|
| `GET` | `/api/products` · `POST` `/api/products` | 品类管理 |
| `POST` | `/api/sales/batch` | 标准层写入（调试/内部用） |
| `GET` | `/api/analytics/sales` | 销售汇总 |
| `GET` | `/api/analytics/top-products` | 热销商品 |
| `GET` | `/api/analytics/brands` | 品牌排行 |
| `GET` | `/api/analytics/monthly-trend` | 月度趋势 |

---

## 8. 数据库表

| 表 | 层 | 说明 |
|---|---|---|
| `import_batches` | 原始层 | 批次元信息：source / category / status / row_count |
| `raw_records` | 原始层 | `row_data` **JSONB**，键即用户表格原始列名 |
| `field_mappings` | 映射层 | 唯一 `(category, source_column)` |
| `sales` | 标准层 | 统一结构，`batch_id` 可溯源 |
| `products` | 品类 | 品类维度（phone / laptop） |

### 批次状态流转

```mermaid
stateDiagram-v2
    [*] --> loaded: POST /api/imports
    loaded --> mapped: POST /api/imports/:id/transform
    loaded --> failed: 转换失败
    mapped --> mapped: 重复转换（幂等）
```

---

## 9. 可扩展性演示

**场景：换个商家，列名不同**

```json
批次 1: {"productor":"Apple", "name":"iPhone 15 Pro", "price":"7114.41", "sales":"11556", "month":"2024-01"}
批次 2: {"厂商":"Apple", "型号":"iPhone 16", "单价":"8000", "月销量":"1234", "月份":"2025-01"}
```

只需为 `category = phone` 补 5 条映射规则：

```
厂商 → productor   型号 → name   单价 → price   月销量 → sales   月份 → month
```

**零代码改动**，两批数据产出完全同构的 `sales` 行。

**场景：加新字段（地区）**

`sales` 加一列 `region`；映射规则加一条 `地区 → region`；分析时 `GROUP BY region`。只加字段，不改逻辑。

**场景：接入 Excel / API**

接入层加一个 `ExcelImporter` / `APIImporter`，后面几层完全复用。这就是「开放扩展」的正确姿势。

---

*本文档随架构演进更新。*
