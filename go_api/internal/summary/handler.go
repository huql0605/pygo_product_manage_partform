package summary

import (
	"github.com/gin-gonic/gin"
	"github.com/huql0605/go_api/internal/common/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// refresh 重建汇总层
func (h *Handler) Refresh(c *gin.Context) {
	res, err := h.svc.Refresh()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, res)
}

// verify 比较汇总层与明细层
func (h *Handler) Verify(c *gin.Context) {
	res, err := h.svc.Verify()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, res)
}
