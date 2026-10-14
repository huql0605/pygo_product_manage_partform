package mapping

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/huql0605/go_api/internal/sale"
)

// ---------- 测试辅助 ----------

// validValues 一份合法的标准字段集合，作为 buildRow 用例的基线。
var validValues = map[string]string{
	"productor": "Apple",
	"name":      "iPhone 15 Pro",
	"price":     "7114.41",
	"sales":     "11556",
	"month":     "2024-01",
}

// withField 复制一份 values 并覆盖某个字段，避免用例之间互相污染。
func withField(base map[string]string, k, v string) map[string]string {
	out := make(map[string]string, len(base))
	for key, val := range base {
		out[key] = val
	}
	out[k] = v
	return out
}

// ---------- resolveRow ----------

func TestResolveRow(t *testing.T) {
	tests := []struct {
		name        string
		row         map[string]any
		rules       map[string]string
		wantValues  map[string]string
		wantIgnored []string
		wantErr     bool
	}{
		{
			name: "恒等回退：列名已是标准字段名（批次 1 场景，零配置）",
			row: map[string]any{
				"productor": "Apple",
				"name":      "iPhone 15 Pro",
				"price":     "7114.41",
				"sales":     "11556",
				"month":     "2024-01",
			},
			wantValues: map[string]string{
				"productor": "Apple",
				"name":      "iPhone 15 Pro",
				"price":     "7114.41",
				"sales":     "11556",
				"month":     "2024-01",
			},
		},
		{
			name: "显式规则：中文列名（批次 2 场景）",
			row: map[string]any{
				"厂商": "Apple", "型号": "iPhone 16",
				"单价": "8000", "月销量": "1234", "月份": "2025-01",
			},
			rules: map[string]string{
				"厂商": "productor", "型号": "name",
				"单价": "price", "月销量": "sales", "月份": "month",
			},
			wantValues: map[string]string{
				"productor": "Apple", "name": "iPhone 16",
				"price": "8000", "sales": "1234", "month": "2025-01",
			},
		},
		{
			name:       "显式规则优先于恒等回退",
			row:        map[string]any{"productor": "Apple"},
			rules:      map[string]string{"productor": "name"},
			wantValues: map[string]string{"name": "Apple"},
		},
		{
			name:        "未映射的列记入 ignored，不报错",
			row:         map[string]any{"productor": "Apple", "地区": "华东"},
			wantValues:  map[string]string{"productor": "Apple"},
			wantIgnored: []string{"地区"},
		},
		{
			name: "歧义：恒等回退与显式规则撞车 → 报错",
			row: map[string]any{
				"productor": "Apple",
				"厂商":        "富士康",
			},
			rules:   map[string]string{"厂商": "productor"},
			wantErr: true,
		},
		{
			name: "歧义：两条显式规则指向同一目标字段 → 报错",
			row: map[string]any{
				"品牌": "Apple",
				"厂商": "富士康",
			},
			rules:   map[string]string{"品牌": "productor", "厂商": "productor"},
			wantErr: true,
		},
		{
			name:       "空行：返回空结果，不报错",
			row:        map[string]any{},
			wantValues: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValues, gotIgnored, err := resolveRow(tt.row, tt.rules)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("期望报错，实际没有；values=%v", gotValues)
				}
				return
			}
			if err != nil {
				t.Fatalf("不期望报错，实际: %v", err)
			}
			if !reflect.DeepEqual(gotValues, tt.wantValues) {
				t.Errorf("values 不符\n got: %v\nwant: %v", gotValues, tt.wantValues)
			}
			sort.Strings(gotIgnored)
			if !reflect.DeepEqual(gotIgnored, tt.wantIgnored) {
				t.Errorf("ignored 不符\n got: %v\nwant: %v", gotIgnored, tt.wantIgnored)
			}
		})
	}
}

// ---------- buildRow ----------

func TestBuildRow(t *testing.T) {
	tests := []struct {
		name    string
		values  map[string]string
		lineNo  int
		want    *sale.SaleRow
		errPart string // 错误信息应包含的片段；空表示不期望报错
	}{
		{
			name:   "正常：字符串正确转成 float/int64",
			values: validValues,
			lineNo: 1,
			want: &sale.SaleRow{
				Productor: "Apple", Name: "iPhone 15 Pro",
				Price: 7114.41, Sales: 11556, Month: "2024-01",
			},
		},
		{
			name:    "缺字段：key 根本不存在",
			values:  map[string]string{"productor": "Apple"},
			lineNo:  3,
			errPart: "第 3 行找不到字段",
		},
		{
			name:    "空值：key 存在但为空串",
			values:  withField(validValues, "productor", ""),
			lineNo:  7,
			errPart: `第 7 行字段 "productor" 为空`,
		},
		{
			name:    "price 不是数字",
			values:  withField(validValues, "price", "七千块"),
			lineNo:  2,
			errPart: "price 不是数字",
		},
		{
			name:    "sales 不是数字",
			values:  withField(validValues, "sales", "一万"),
			lineNo:  2,
			errPart: "sales 不是数字",
		},
		{
			name:    "month 缺前导零",
			values:  withField(validValues, "month", "2024-1"),
			lineNo:  5,
			errPart: "month 格式错误",
		},
		{
			name:    "month 用斜杠",
			values:  withField(validValues, "month", "2024/01"),
			lineNo:  5,
			errPart: "month 格式错误",
		},
		{
			name:    "month 月份越界",
			values:  withField(validValues, "month", "2024-13"),
			lineNo:  5,
			errPart: "month 格式错误",
		},
		{
			name:    "month 只有两位年份",
			values:  withField(validValues, "month", "24-01"),
			lineNo:  5,
			errPart: "month 格式错误",
		},
		{
			name:   "month 边界：12 月合法",
			values: withField(validValues, "month", "2024-12"),
			lineNo: 1,
			want: &sale.SaleRow{
				Productor: "Apple", Name: "iPhone 15 Pro",
				Price: 7114.41, Sales: 11556, Month: "2024-12",
			},
		},
		{
			name:   "sales 是小数：截断为整数",
			values: withField(validValues, "sales", "11556.9"),
			lineNo: 1,
			want: &sale.SaleRow{
				Productor: "Apple", Name: "iPhone 15 Pro",
				Price: 7114.41, Sales: 11556, Month: "2024-01",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildRow(tt.values, tt.lineNo)

			if tt.errPart != "" {
				if err == nil {
					t.Fatalf("期望报错（含 %q），实际没有", tt.errPart)
				}
				if !strings.Contains(err.Error(), tt.errPart) {
					t.Fatalf("错误信息不符\n got: %v\nwant 含: %s", err, tt.errPart)
				}
				return
			}
			if err != nil {
				t.Fatalf("不期望报错，实际: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SaleRow 不符\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

// ---------- valueToString ----------

func TestValueToString(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"nil 转空串", nil, ""},
		{"字符串原样保留", "Apple", "Apple"},
		{"字符串去除首尾空格", "  Apple  ", "Apple"},
		{"中文原样保留", "小米14 Ultra", "小米14 Ultra"},
		{"JSON 整数不出现小数点", float64(8000), "8000"},
		{"较大的 JSON 整数", float64(11556), "11556"},
		{"零值", float64(0), "0"},
		{"小数保持精度", 7114.41, "7114.41"},
		{"布尔", true, "true"},
		{"未覆盖类型走 fmt.Sprint", 42, "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := valueToString(tt.in); got != tt.want {
				t.Errorf("valueToString(%#v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------- decodeRow ----------

func TestDecodeRow(t *testing.T) {
	t.Run("字符串值", func(t *testing.T) {
		m, err := decodeRow([]byte(`{"productor":"Apple","price":"7114.41"}`))
		if err != nil {
			t.Fatalf("不期望报错: %v", err)
		}
		if m["productor"] != "Apple" || m["price"] != "7114.41" {
			t.Errorf("解析结果不符: %v", m)
		}
	})

	t.Run("JSON 数字解析成 float64（与 valueToString 配合的关键点）", func(t *testing.T) {
		m, err := decodeRow([]byte(`{"单价":8000,"月销量":1234}`))
		if err != nil {
			t.Fatalf("不期望报错: %v", err)
		}
		if v, ok := m["单价"].(float64); !ok || v != 8000 {
			t.Errorf("期望 float64(8000)，实际 %#v", m["单价"])
		}
	})

	t.Run("非法 JSON 应报错", func(t *testing.T) {
		if _, err := decodeRow([]byte(`{bad`)); err == nil {
			t.Error("期望报错，实际没有")
		}
	})
}

// ---------- IsStandardField ----------

func TestIsStandardField(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"productor", true},
		{"name", true},
		{"price", true},
		{"sales", true},
		{"month", true},
		{"Productor", false}, // 大小写敏感
		{" name", false},     // 不自动去空格
		{"region", false},    // 白名单外
		{"品牌", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := IsStandardField(tt.in); got != tt.want {
				t.Errorf("IsStandardField(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// ---------- sortedKeys ----------

func TestSortedKeys(t *testing.T) {
	t.Run("乱序输入 → 有序输出", func(t *testing.T) {
		got := sortedKeys(map[string]struct{}{"c": {}, "a": {}, "b": {}})
		want := []string{"a", "b", "c"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("空 map → 长度为 0", func(t *testing.T) {
		if got := sortedKeys(map[string]struct{}{}); len(got) != 0 {
			t.Errorf("期望空切片，实际 %v", got)
		}
	})
}
