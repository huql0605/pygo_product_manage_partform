package raw

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/huql0605/go_api/internal/common/errors"
	"github.com/huql0605/go_api/internal/common/response"
)

type Handler struct {
	svc *Service
}

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
