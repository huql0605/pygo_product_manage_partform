package product

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

func (h *Handler) List(c *gin.Context) {

	items, err := h.svc.List()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, items)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateProductReq

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, errors.CodeInvalidParam, err.Error())
		return
	}
	p := Product{
		Name: req.Name,
	}
	if err := h.svc.Create(&p); err != nil {
		response.BadRequest(c, errors.CodeInvalidParam, err.Error())
		return
	}
	response.Success(c, p)
}
