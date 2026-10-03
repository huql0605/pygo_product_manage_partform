package analytics

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	client *Client
}

func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

func (h *Handler) Sales(c *gin.Context) {
	days, err := strconv.Atoi(c.Query("days"))
	if err != nil || days <= 0 {
		days = 7
	}

	data, err := h.client.Sales(c.Request.Context(), days)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)

}

func (h *Handler) TopProducts(c *gin.Context) {
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil || limit <= 0 {
		limit = 5
	}
	data, err := h.client.TopProducts(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) Brands(c *gin.Context) {
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil || limit <= 0 {
		limit = 5
	}
	data, err := h.client.Brands(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) MonthlyTrend(c *gin.Context) {
	productor := c.Query("productor")
	data, err := h.client.MonthlyTrend(c.Request.Context(), productor)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, data)
}
