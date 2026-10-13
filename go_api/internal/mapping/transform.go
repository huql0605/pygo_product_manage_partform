package mapping

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/huql0605/go_api/internal/sale"
)

var monthRe = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

func decodeRow(data []byte) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// resolveRow 对一行原始数据求值，返回「标准字段 → 值」+ 被忽略的列。
//
// 解析优先级：
//  1. 显式规则     source_column -> target_field
//  2. 恒等回退     源列名本身就是标准字段名（让批次 1 零配置）
//  3. 都不匹配     记入 ignored —— 数据不丢，只是本次转换用不上

func resolveRow(row map[string]any, rules map[string]string) (map[string]string, []string) {
	values := make(map[string]string, len(StandardFields))
	var ignored []string

	for col, raw := range row {
		target := rules[col]
		if target == "" && IsStandardField(col) {
			target = col
		}
		if target == "" {
			ignored = append(ignored, col)
			continue
		}
		if _, exists := values[target]; !exists {
			values[target] = valueToString(raw)
		}

	}
	return values, ignored

}

func valueToString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))

	}
}

func buildRow(values map[string]string, lineNo int) (*sale.SaleRow, error) {
	for _, f := range StandardFields {
		v, ok := values[f]
		if !ok {
			return nil, fmt.Errorf("第 %d 行找不到字段 %q 的来源列", lineNo, f)
		}
		if v == "" {
			return nil, fmt.Errorf("第 %d 行字段 %q 为空", lineNo, f)
		}
	}
	price, err := strconv.ParseFloat(values["price"], 64)
	if err != nil {
		return nil, fmt.Errorf("第 %d 行 price 不是数字: %q", lineNo, values["price"])
	}
	salesNum, err := strconv.ParseFloat(values["sales"], 64)
	if err != nil {
		return nil, fmt.Errorf("第 %d 行 sales 不是数字: %q", lineNo, values["sales"])
	}
	if !monthRe.MatchString(values["month"]) {
		return nil, fmt.Errorf("第 %d 行 month 格式错误: %q (需要 YYYY-MM)", lineNo, values["month"])
	}
	return &sale.SaleRow{
		Productor: values["productor"],
		Name:      values["name"],
		Price:     price,
		Sales:     int64(salesNum),
		Month:     values["month"],
	}, nil
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
