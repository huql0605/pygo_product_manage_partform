package raw

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/huql0605/go_api/internal/common/errors"
	"github.com/huql0605/go_api/internal/common/response"
)

type Handler struct {
	svc *Service
}

const (
	maxUploadBytes  = 32 << 20
	maxRequestBytes = maxUploadBytes + 16<<20
	maxImportRows   = 10000
)

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) CreateImport(c *gin.Context) {
	var req CreateImportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, errors.CodeInvalidParam, err.Error())
		return
	}
	if err := req.Validate(); err != nil {
		response.BadRequest(c, errors.CodeInvalidParam, err.Error())
		return
	}
	res, err := h.svc.CreateBatch(req.Source, req.Category, req.Rows)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, res)
}

func (h *Handler) ListColumns(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, errors.CodeInvalidParam, "非法的批次 id")
		return
	}
	cols, err := h.svc.ListColumns(uint(id))
	if err == ErrBatchNotFound {
		response.NotFound(c, err.Error())
		return
	}
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, cols)
}

func (h *Handler) ShowImportsCount(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, errors.CodeInvalidParam, "非法的批次 id")
		return
	}
	count, err := h.svc.CountRecords(uint(id))
	if err == ErrBatchNotFound {
		response.NotFound(c, err.Error())
		return
	}
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, count)
}

// ListImports 分页列出批次，可按 category / status 过滤。
func (h *Handler) ListImports(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	res, err := h.svc.ListBatches(c.Query("category"), c.Query("status"), page, size)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, res)
}

// DeleteImport 删除批次及其明细行、原始数据，并重建汇总层。
func (h *Handler) DeleteImport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, errors.CodeInvalidParam, "非法的批次 id")
		return
	}

	res, err := h.svc.DeleteBatch(uint(id))
	if err == ErrBatchNotFound {
		response.NotFound(c, err.Error())
		return
	}
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, res)
}

func (h *Handler) UploadImport(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBytes)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, errors.CodeInvalidParam, "缺少 file 字段")
		return
	}
	if fileHeader.Size > maxUploadBytes {
		response.BadRequest(c, errors.CodeInvalidParam, "文件超过大小限制")
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxUploadBytes+1))
	if err != nil {
		response.BadRequest(c, errors.CodeInvalidParam, "读取文件失败："+err.Error())
		return
	}
	if len(data) == 0 {
		response.BadRequest(c, errors.CodeInvalidParam, "empty file")
		return
	}

	if len(data) > maxUploadBytes {
		response.BadRequest(c, errors.CodeInvalidParam, "too big file ")
		return
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(c.PostForm("rows")), &rows); err != nil {
		response.BadRequest(c, errors.CodeInvalidParam, "rows is  invalid json "+err.Error())
		return
	}

	if len(rows) == 0 || len(rows) > maxImportRows {
		response.BadRequest(c, errors.CodeInvalidParam, "rows count must between 1 and 10000")
		return
	}
	req := CreateImportReq{
		Source:   c.PostForm("source"),
		Category: c.PostForm("category"),
		Rows:     rows,
	}

	if req.Source == "" {
		req.Source = fileHeader.Filename
	}
	if err := req.Validate(); err != nil {
		response.BadRequest(c, errors.CodeInvalidParam, err.Error())
		return
	}
	if len(req.Source) > 255 || len(req.Category) > 50 {
		response.BadRequest(c, errors.CodeInvalidParam, "source or category is too long")
		return
	}

	res, err := h.svc.CreateBatchFromFile(UploadImportInput{
		Source:        req.Source,
		Category:      req.Category,
		Filename:      fileHeader.Filename,
		ContentType:   fileHeader.Header.Get("Content-Type"),
		ReaderVersion: c.PostForm("reader_version"),
		Content:       data,
		Rows:          req.Rows,
		ExpectedHash:  c.PostForm("file_hash"),
	})

	if err != nil {
		response.BadRequest(c, errors.CodeInvalidParam, err.Error())
		return
	}
	response.Success(c, res)
}
