package mapping

// CreateMappingReq 配置一条映射规则。重复提交同 (category, source_column) 会覆盖 target_field。
type CreateMappingReq struct {
	Category     string `json:"category" binding:"required,min=1,max=50"`
	SourceColumn string `json:"source_column" binding:"required,min=1,max=100"`
	TargetField  string `json:"target_field" binding:"required,min=1,max=50"`
}
