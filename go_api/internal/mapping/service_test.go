package mapping

import (
	"errors"
	"strings"
	"testing"

	"github.com/huql0605/go_api/internal/raw"
	"github.com/huql0605/go_api/internal/sale"
	"gorm.io/gorm"
)

// ---------- 假实现：Transform 的三個依赖都可以替换，无需数据库 ----------

type fakeRules struct{ items []FieldMapping }

func (f *fakeRules) Upsert(m *FieldMapping) error { return nil }

func (f *fakeRules) ListByCategory(category string) ([]FieldMapping, error) {
	return f.items, nil
}

type fakeRaw struct {
	batch     *raw.ImportBatch
	batchErr  error
	records   []raw.RawRecord
	statusLog []string
}

func (f *fakeRaw) GetBatch(id uint) (*raw.ImportBatch, error) {
	if f.batchErr != nil {
		return nil, f.batchErr
	}
	return f.batch, nil
}

func (f *fakeRaw) ListRecords(batchID uint) ([]raw.RawRecord, error) {
	return f.records, nil
}

func (f *fakeRaw) UpdateStatus(batchID uint, status string) error {
	f.statusLog = append(f.statusLog, status)
	return nil
}

type fakeSale struct {
	calls    int
	gotBatch uint
	gotRows  []sale.SaleRow
	err      error
}

func (f *fakeSale) ReplaceBatch(product string, batchID uint, rows []sale.SaleRow) (*sale.CreateBatchResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	f.gotBatch = batchID
	f.gotRows = rows
	id := batchID
	return &sale.CreateBatchResult{ProductID: 1, BatchID: &id, Inserted: len(rows)}, nil
}

// ---------- 测试辅助 ----------

func newSvc(rules []FieldMapping, fr *fakeRaw, fs *fakeSale) *Service {
	return NewService(&fakeRules{items: rules}, fr, fs)
}

func rec(idx int, body string) raw.RawRecord {
	return raw.RawRecord{RowIndex: idx, RowData: []byte(body)}
}

const fullRow = `{"productor":"Apple","name":"iPhone 15","price":"7114.41","sales":"11556","month":"2024-01"}`

func chineseRules() []FieldMapping {
	return []FieldMapping{
		{Category: "phone", SourceColumn: "厂商", TargetField: "productor"},
		{Category: "phone", SourceColumn: "型号", TargetField: "name"},
		{Category: "phone", SourceColumn: "单价", TargetField: "price"},
		{Category: "phone", SourceColumn: "月销量", TargetField: "sales"},
		{Category: "phone", SourceColumn: "月份", TargetField: "month"},
	}
}

// ---------- 用例 ----------

func TestTransform_SuccessMarksMapped(t *testing.T) {
	fr := &fakeRaw{
		batch:   &raw.ImportBatch{ID: 1, Category: "phone", Status: raw.StatusLoaded},
		records: []raw.RawRecord{rec(0, fullRow), rec(1, fullRow)},
	}
	fs := &fakeSale{}

	res, err := newSvc(nil, fr, fs).Transform(1)
	if err != nil {
		t.Fatalf("不期望报错: %v", err)
	}

	if res.Inserted != 2 {
		t.Errorf("Inserted = %d, want 2", res.Inserted)
	}
	if len(fs.gotRows) != 2 {
		t.Fatalf("写入行数 = %d, want 2", len(fs.gotRows))
	}
	if fs.gotRows[0].Productor != "Apple" || fs.gotRows[0].Price != 7114.41 || fs.gotRows[0].Sales != 11556 {
		t.Errorf("类型转换或取值不对: %+v", fs.gotRows[0])
	}
	if fs.gotBatch != 1 {
		t.Errorf("batchID = %d, want 1", fs.gotBatch)
	}
	if len(res.IgnoredColumns) != 0 {
		t.Errorf("IgnoredColumns = %v, want 空", res.IgnoredColumns)
	}
	if len(fr.statusLog) != 1 || fr.statusLog[0] != raw.StatusMapped {
		t.Errorf("状态流转 = %v, want [mapped]", fr.statusLog)
	}
}

// 守卫 BUG 1：歧义必须报出「多个来源列」，而不是误导性的「找不到字段」
func TestTransform_AmbiguousColumns_ReportsRealError(t *testing.T) {
	rules := append(chineseRules(),
		FieldMapping{Category: "phone", SourceColumn: "品牌", TargetField: "productor"},
	)
	// 同一行里 品牌 和 厂商 都指向 productor
	row := `{"品牌":"Apple","厂商":"富士康","型号":"x","单价":"1","月销量":"1","月份":"2024-01"}`

	fr := &fakeRaw{
		batch:   &raw.ImportBatch{ID: 9, Category: "phone"},
		records: []raw.RawRecord{rec(0, row)},
	}
	fs := &fakeSale{}

	_, err := newSvc(rules, fr, fs).Transform(9)
	if err == nil {
		t.Fatal("期望报错，实际没有")
	}
	if !strings.Contains(err.Error(), "多个来源列") {
		t.Errorf("错误信息应指出「多个来源列」，实际: %v", err)
	}
	if strings.Contains(err.Error(), "找不到字段") {
		t.Errorf("不该出现误导性的「找不到字段」: %v", err)
	}
	if fs.calls != 0 {
		t.Errorf("歧义时不该写库，实际调用 ReplaceBatch %d 次", fs.calls)
	}
	if len(fr.statusLog) != 1 || fr.statusLog[0] != raw.StatusFailed {
		t.Errorf("状态应流转到 failed，实际 %v", fr.statusLog)
	}
}

// 守卫 BUG 2：转换失败必须把批次标成 failed，且不写库
func TestTransform_MissingField_MarksFailed(t *testing.T) {
	rules := []FieldMapping{
		{Category: "phone", SourceColumn: "品牌", TargetField: "productor"},
		{Category: "phone", SourceColumn: "单价", TargetField: "price"},
		{Category: "phone", SourceColumn: "月销量", TargetField: "sales"},
		{Category: "phone", SourceColumn: "月份", TargetField: "month"},
	}
	// 少了 name
	row := `{"品牌":"Apple","单价":"1","月销量":"1","月份":"2024-01"}`

	fr := &fakeRaw{
		batch:   &raw.ImportBatch{ID: 4, Category: "phone"},
		records: []raw.RawRecord{rec(0, row)},
	}
	fs := &fakeSale{}

	_, err := newSvc(rules, fr, fs).Transform(4)
	if err == nil {
		t.Fatal("期望报错，实际没有")
	}
	if !strings.Contains(err.Error(), "找不到字段") {
		t.Errorf("错误应指出缺失的字段，实际: %v", err)
	}
	if len(fr.statusLog) != 1 || fr.statusLog[0] != raw.StatusFailed {
		t.Errorf("状态应流转到 failed，实际 %v", fr.statusLog)
	}
	if fs.calls != 0 {
		t.Errorf("失败时不该写库，实际调用 %d 次", fs.calls)
	}
}

func TestTransform_ExplicitRulesTranslateChineseColumns(t *testing.T) {
	fr := &fakeRaw{
		batch: &raw.ImportBatch{ID: 2, Category: "phone"},
		records: []raw.RawRecord{
			rec(0, `{"厂商":"Apple","型号":"iPhone 16","单价":"8000","月销量":"1234","月份":"2025-01"}`),
		},
	}
	fs := &fakeSale{}

	res, err := newSvc(chineseRules(), fr, fs).Transform(2)
	if err != nil {
		t.Fatalf("不期望报错: %v", err)
	}
	if res.Inserted != 1 {
		t.Fatalf("Inserted = %d, want 1", res.Inserted)
	}

	got := fs.gotRows[0]
	if got.Productor != "Apple" || got.Name != "iPhone 16" ||
		got.Price != 8000 || got.Sales != 1234 || got.Month != "2025-01" {
		t.Errorf("中文列名翻译结果不对: %+v", got)
	}
}

func TestTransform_IgnoredColumnsReported(t *testing.T) {
	fr := &fakeRaw{
		batch: &raw.ImportBatch{ID: 1, Category: "phone"},
		records: []raw.RawRecord{
			rec(0, `{"productor":"Apple","name":"x","price":"1","sales":"1","month":"2024-01","地区":"华东"}`),
		},
	}
	fs := &fakeSale{}

	res, err := newSvc(nil, fr, fs).Transform(1)
	if err != nil {
		t.Fatalf("不期望报错: %v", err)
	}
	if len(res.IgnoredColumns) != 1 || res.IgnoredColumns[0] != "地区" {
		t.Errorf("IgnoredColumns = %v, want [地区]", res.IgnoredColumns)
	}
}

func TestTransform_NoRecords(t *testing.T) {
	fr := &fakeRaw{batch: &raw.ImportBatch{ID: 5, Category: "phone"}}

	_, err := newSvc(nil, fr, &fakeSale{}).Transform(5)
	if err == nil || !strings.Contains(err.Error(), "没有原始数据") {
		t.Errorf("期望「没有原始数据」，实际: %v", err)
	}
}

// 批次不存在时，不该去更新一个不存在的批次
func TestTransform_BatchNotFound(t *testing.T) {
	fr := &fakeRaw{batchErr: gorm.ErrRecordNotFound}

	_, err := newSvc(nil, fr, &fakeSale{}).Transform(999)
	if !errors.Is(err, raw.ErrBatchNotFound) {
		t.Errorf("期望 ErrBatchNotFound，实际: %v", err)
	}
	if len(fr.statusLog) != 0 {
		t.Errorf("批次不存在时不该写状态，实际 %v", fr.statusLog)
	}
}

// 幂等性：重复转换仍走 ReplaceBatch（真正的幂等由 sale 层 DELETE+INSERT 保证）
func TestTransform_RepeatableCallsReplaceBatch(t *testing.T) {
	fr := &fakeRaw{
		batch:   &raw.ImportBatch{ID: 1, Category: "phone"},
		records: []raw.RawRecord{rec(0, fullRow)},
	}
	fs := &fakeSale{}
	svc := newSvc(nil, fr, fs)

	for i := 1; i <= 3; i++ {
		if _, err := svc.Transform(1); err != nil {
			t.Fatalf("第 %d 次调用失败: %v", i, err)
		}
	}
	if fs.calls != 3 {
		t.Errorf("ReplaceBatch 调用次数 = %d, want 3", fs.calls)
	}
	if last := fr.statusLog[len(fr.statusLog)-1]; last != raw.StatusMapped {
		t.Errorf("最终状态 = %s, want mapped", last)
	}
}
