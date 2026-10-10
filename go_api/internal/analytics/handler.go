package analytics

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/huql0605/go_api/internal/common/errors"
	"github.com/huql0605/go_api/internal/common/response"
)

type Handler struct {
	client *Client
}

func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

func (h *Handler) Sales(c *gin.Context) {
	startMonth := c.Query("start_month")
	endMonth := c.Query("end_month")
	data, err := h.client.Sales(c.Request.Context(), startMonth, endMonth)
	if err != nil {
		response.BadGateway(c, errors.CodePythonDown, err.Error())
		return
	}
	response.Success(c, data)

}

func (h *Handler) TopProducts(c *gin.Context) {
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil || limit <= 0 {
		limit = 5
	}
	data, err := h.client.TopProducts(c.Request.Context(), limit)
	if err != nil {
		response.BadGateway(c, errors.CodePythonDown, err.Error())
		return
	}
	response.Success(c, data)
}

func (h *Handler) Brands(c *gin.Context) {
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil || limit <= 0 {
		limit = 5
	}
	data, err := h.client.Brands(c.Request.Context(), limit)
	if err != nil {
		response.BadGateway(c, errors.CodePythonDown, err.Error())
		return
	}
	response.Success(c, data)
}

func (h *Handler) MonthlyTrend(c *gin.Context) {
	productor := c.Query("productor")
	data, err := h.client.MonthlyTrend(c.Request.Context(), productor)
	if err != nil {
		response.BadGateway(c, errors.CodePythonDown, err.Error())
		return
	}
	response.Success(c, data)
}
