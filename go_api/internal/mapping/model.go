package mapping

import "time"

// StandardFields 是标准层（sales 表）允许的目标字段白名单。
// 映射规则只能把用户列名翻译成这些字段，防止写错列。
var StandardFields = []string{"productor", "name", "price", "sales", "month"}

// RequiredFields 转换时必须有值的字段。标准字段可以只进白名单不进这里，
// 那样它就是可选字段，缺失不会导致整批失败。
var RequiredFields = []string{"productor", "name", "price", "sales", "month"}

func IsStandardField(f string) bool {
	for _, s := range StandardFields {
		if s == f {
			return true
		}
	}
	return false
}

// FieldMapping 一条「用户列名 → 标准字段」的翻译规则。
//
// 注意：唯一约束是 (category, source_column)，target_field 故意不加唯一约束——
// 因为不同来源可能用不同的写法指向同一个目标字段（如「productor」和「厂商」都 → productor）。
// 转换时按「这一行里实际出现了哪个源列」来解析。
type FieldMapping struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	Category     string    `json:"category" gorm:"not null;uniqueIndex:uk_mapping_cat_col,priority:1"`
	SourceColumn string    `json:"source_column" gorm:"not null;uniqueIndex:uk_mapping_cat_col,priority:2"`
	TargetField  string    `json:"target_field" gorm:"not null"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
