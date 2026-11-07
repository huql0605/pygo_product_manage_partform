package summary

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

// Refresh 按批次重算汇总层。
func (h *Handler) Refresh(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, errors.CodeInvalidParam, "非法的批次 id")
		return
	}
	res, err := h.svc.Refresh(uint(id))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, res)
}

// Verify 比较汇总层与明细层。
func (h *Handler) Verify(c *gin.Context) {
	res, err := h.svc.Verify()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, res)
}

// RefreshAll 全量重算，作为手工兜底。
func (h *Handler) RefreshAll(c *gin.Context) {
	res, err := h.svc.RefreshAll()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, res)
}
