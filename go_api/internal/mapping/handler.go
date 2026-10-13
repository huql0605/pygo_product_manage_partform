package mapping

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/huql0605/go_api/internal/common/errors"
	"github.com/huql0605/go_api/internal/common/response"
	"github.com/huql0605/go_api/internal/raw"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Upsert 配置（或覆盖）一条映射规则。
func (h *Handler) Upsert(c *gin.Context) {
	var req CreateMappingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, errors.CodeInvalidParam, err.Error())
		return
	}
	m, err := h.svc.Upsert(req)
	if err != nil {
		response.BadRequest(c, errors.CodeInvalidParam, err.Error())
		return
	}
	response.Success(c, m)
}

// List 查看某品类的全部映射规则。
func (h *Handler) List(c *gin.Context) {
	category := c.Query("category")
	if category == "" {
		response.BadRequest(c, errors.CodeInvalidParam, "缺少 category 参数")
		return
	}
	items, err := h.svc.ListByCategory(category)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, items)
}

func (h *Handler) Transform(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, errors.CodeInvalidParam, "非法的批次 id")
		return
	}
	res, err := h.svc.Transform(uint(id))
	if err == raw.ErrBatchNotFound {
		response.NotFound(c, err.Error())
		return
	}
	if err != nil {
		response.BadRequest(c, errors.CodeInvalidParam, err.Error())
		return
	}
	response.Success(c, res)
}
