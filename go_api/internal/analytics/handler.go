package analytics

import (
	stdErrors "errors"
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

func filterFrom(c *gin.Context) Filter {
	return Filter{
		Category:   c.Query("category"),
		Productor:  c.Query("productor"),
		StartMonth: c.Query("start_month"),
		EndMonth:   c.Query("end_month"),
	}
}

func (h *Handler) Sales(c *gin.Context) {
	f := filterFrom(c)
	data, err := h.client.Sales(c.Request.Context(), f)
	if err != nil {
		if stdErrors.Is(err, ErrUpstreamNotFound) {
			response.NotFound(c, err.Error())
			return
		}
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
	f := filterFrom(c)
	data, err := h.client.TopProducts(c.Request.Context(), limit, f)
	if err != nil {
		if stdErrors.Is(err, ErrUpstreamNotFound) {
			response.NotFound(c, err.Error())
			return
		}
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
	f := filterFrom(c)
	data, err := h.client.Brands(c.Request.Context(), limit, f)
	if err != nil {
		if stdErrors.Is(err, ErrUpstreamNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.BadGateway(c, errors.CodePythonDown, err.Error())
		return
	}
	response.Success(c, data)
}

func (h *Handler) MonthlyTrend(c *gin.Context) {
	f := filterFrom(c)
	data, err := h.client.MonthlyTrend(c.Request.Context(), f)
	if err != nil {
		if stdErrors.Is(err, ErrUpstreamNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.BadGateway(c, errors.CodePythonDown, err.Error())
		return
	}
	response.Success(c, data)
}
