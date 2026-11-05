package sale

import (
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

func (h *Handler) CreateBatch(c *gin.Context) {
	var req BatchCreateSalesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, errors.CodeInvalidParam, err.Error())
		return
	}
	res, err := h.svc.CreateBatch(req.Product, req.Rows)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, res)
}
