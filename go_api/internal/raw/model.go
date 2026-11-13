package raw

import (
	"encoding/json"
	"time"
)

const (
	// StatusLoaded 批次已落库，尚未映射到明细层。
	StatusLoaded = "loaded"
	// StatusMapped 批次已完成映射，明细层有对应的 sales 行。
	StatusMapped = "mapped"
	// StatusFailed 历史失败状态；当前转换失败时保持原状态，只有旧数据可能残留该值。
	StatusFailed = "failed"
)

// ImportBatch 是一次导入的批次元信息，对应表 import_batches。
// 批次与原始文件、原始行、明细行之间都通过 ID 关联，是数据溯源的中间节点。
type ImportBatch struct {
	// ID 自增主键。
	ID uint `json:"id" gorm:"primarykey"`
	// Source 来源标识，通常是上传文件名，手工批次为 manual。
	Source string `json:"source" gorm:"not null"`
	// Category 品类（取 products.name）；与 raw_file_id 组成唯一键 uk_batch_cat_file。
	Category string `json:"category" gorm:"not null;index;uniqueIndex:uk_batch_cat_file,priority:1"`
	// Status 批次状态，取值见 StatusLoaded / StatusMapped / StatusFailed。
	Status string `json:"status" gorm:"not null;default:loaded;index"`
	// RowCount 原始行数，与 raw_records 的行数一致。
	RowCount int `json:"row_count" gorm:"not null;default:0"`
	// FileHash 原始文件 SHA-256，冗余自 raw_files.sha256，便于按文件反查批次。
	FileHash string `json:"file_hash" gorm:"type:char(64);not null;default:'';index"`
	// RawFileID 指向 raw_files.id，可空；同一文件同一品类只建一个批次，
	// 重复上传直接复用已有批次（duplicated=true）。
	RawFileID *uint `json:"raw_file_id" gorm:"uniqueIndex:uk_batch_cat_file,priority:2"`
	// ReaderVersion 生成该批次原始行时使用的解析器版本，便于解析规则升级后区分来源。
	ReaderVersion string `json:"reader_version" gorm:"type:varchar(32);not null;default:''"`
	// CreatedAt 创建时间，由 GORM 自动维护。
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间，由 GORM 自动维护。
	UpdatedAt time.Time `json:"updated_at"`
}

// RawRecord 是原始行层的一条解析快照，对应表 raw_records。
// 键保持源表列名原样，不做任何标准化，可随时从 raw_files 重新解析生成。
type RawRecord struct {
	// ID 自增主键。
	ID uint `json:"id" gorm:"primarykey"`
	// BatchID 所属批次，指向 import_batches.id；删除批次时由应用层一并清理。
	BatchID uint `json:"batch_id" gorm:"not null;index"`
	// RowIndex 行序号，从 0 开始；返回错误时加 1 转成用户可见行号。
	RowIndex int `json:"row_index" gorm:"not null"`
	// RowData 该行的原始键值，JSONB；键为源表列名，值保留 JSON 原始类型。
	RowData json.RawMessage `json:"row_data" gorm:"type:jsonb;not null"`
	// CreatedAt 创建时间，由 GORM 自动维护；没有显式 json tag，
	// 且 RawRecord 不作为接口返回值，仅内部转换时读取。
	CreatedAt time.Time
}

// RawFile 保存原始文件字节与指纹，对应表 raw_files。
// 同一份内容（sha256 相同）全局只存一份，是重放和去重的依据。
type RawFile struct {
	// ID 自增主键。
	ID uint `json:"id" gorm:"primarykey"`
	// SHA256 文件内容哈希，唯一；由 Go 服务端重新计算并与客户端声明值校验。
	SHA256 string `json:"sha256" gorm:"type:char(64);not null;uniqueIndex"`
	// Size 文件字节数。
	Size int64 `json:"size" gorm:"not null"`
	// Filename 原始文件名。
	Filename string `json:"filename" gorm:"type:text;not null"`
	// ContentType 上传时的 MIME 类型，如 text/csv。
	ContentType string `json:"content_type" gorm:"type:text;not null;default:''"`
	// StorageKind 字节存放方式；当前固定为 db，表示存于 content 列，为对象存储预留。
	StorageKind string `json:"storage_kind" gorm:"type:varchar(16);not null;default:'db'"`
	// ObjectKey 对象存储中的键；StorageKind=db 时为空字符串。
	ObjectKey string `json:"object_key" gorm:"type:text;not null;default:''"`
	// Content 原始文件字节，bytea；json:"-" 保证不会随接口返回。
	Content []byte `json:"-" gorm:"type:bytea"`
	// CreatedAt 创建时间，由 GORM 自动维护。
	CreatedAt time.Time `json:"created_at"`
}

func (RawFile) TableName() string {
	return "raw_files"
}
